package portal

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/mmolotov/cx_hac_ccv2_helper/internal/config"
)

const (
	sessionTTLSec  = 3600
	sessionsSubDir = "sessions"
)

// SessionCache is the on-disk representation of a cached portal session.
type SessionCache struct {
	Cookies   []*SerializableCookie `json:"cookies"`
	ExpiresAt time.Time             `json:"expires_at"`
}

// SerializableCookie is a JSON-serialisable subset of http.Cookie.
type SerializableCookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain"`
	Path     string    `json:"path"`
	Secure   bool      `json:"secure"`
	HttpOnly bool      `json:"http_only"`
	Expires  time.Time `json:"expires,omitempty"`
}

func sessionFilePath(project, subscription string) (string, error) {
	dir, err := config.ConfigDirPath()
	if err != nil {
		return "", err
	}
	sessDir := filepath.Join(dir, sessionsSubDir)
	return filepath.Join(sessDir, fmt.Sprintf("%s_%s.json", project, subscription)), nil
}

// LoadSession loads a cached session from disk. Returns (nil, nil) if the file
// does not exist or the session has expired.
func LoadSession(project, subscription string) (*SessionCache, error) {
	path, err := sessionFilePath(project, subscription)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading session file: %w", err)
	}

	var sess SessionCache
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("parsing session file: %w", err)
	}

	if time.Now().After(sess.ExpiresAt) {
		return nil, nil // expired
	}
	return &sess, nil
}

// SaveSession writes the current cookies from client's jar to the session file.
func SaveSession(client *PortalClient, project, subscription string) error {
	path, err := sessionFilePath(project, subscription)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating sessions dir: %w", err)
	}

	// Collect cookies from the jar for all relevant domains.
	rawCookies := collectAllCookies(client.jar)
	serializable := make([]*SerializableCookie, 0, len(rawCookies))
	for _, c := range rawCookies {
		serializable = append(serializable, cookieToSerializable(c))
	}

	sess := SessionCache{
		Cookies:   serializable,
		ExpiresAt: time.Now().Add(sessionTTLSec * time.Second),
	}

	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling session: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing session file: %w", err)
	}
	return nil
}

// DeleteSession removes the session file for the given project/subscription.
func DeleteSession(project, subscription string) error {
	path, err := sessionFilePath(project, subscription)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// RestoreSession injects cached cookies back into the client's cookie jar.
func RestoreSession(client *PortalClient, sess *SessionCache) error {
	for _, sc := range sess.Cookies {
		u := &url.URL{
			Scheme: "https",
			Host:   sc.Domain,
			Path:   sc.Path,
		}
		client.jar.SetCookies(u, []*http.Cookie{serializableToCookie(sc)})
	}
	return nil
}

// collectAllCookies extracts cookies from the jar by probing known portal domains.
func collectAllCookies(jar interface {
	Cookies(u *url.URL) []*http.Cookie
}) []*http.Cookie {
	domains := []string{
		"portal.commerce.ondemand.com",
		"accounts.sap.com",
	}
	seen := make(map[string]bool)
	var all []*http.Cookie
	for _, domain := range domains {
		for _, scheme := range []string{"https"} {
			u := &url.URL{Scheme: scheme, Host: domain, Path: "/"}
			for _, c := range jar.Cookies(u) {
				key := c.Name + "@" + domain
				if !seen[key] {
					seen[key] = true
					c.Domain = domain
					all = append(all, c)
				}
			}
		}
	}
	return all
}

func cookieToSerializable(c *http.Cookie) *SerializableCookie {
	return &SerializableCookie{
		Name:     c.Name,
		Value:    c.Value,
		Domain:   c.Domain,
		Path:     c.Path,
		Secure:   c.Secure,
		HttpOnly: c.HttpOnly,
		Expires:  c.Expires,
	}
}

func serializableToCookie(sc *SerializableCookie) *http.Cookie {
	return &http.Cookie{
		Name:     sc.Name,
		Value:    sc.Value,
		Domain:   sc.Domain,
		Path:     sc.Path,
		Secure:   sc.Secure,
		HttpOnly: sc.HttpOnly,
		Expires:  sc.Expires,
	}
}
