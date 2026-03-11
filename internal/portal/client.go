package portal

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"time"

	"golang.org/x/net/publicsuffix"
)

// PortalClient wraps an http.Client with a persistent cookie jar.
type PortalClient struct {
	http  *http.Client
	jar   *cookiejar.Jar
	Debug bool
}

// NewPortalClient creates a PortalClient with TLS verification disabled
// (matching the Python helper behaviour for corp proxy environments).
func NewPortalClient() (*PortalClient, error) {
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // intentional, matches Python helper
		Proxy:           http.ProxyFromEnvironment,
	}

	client := &http.Client{
		Transport: transport,
		Jar:       jar,
		Timeout:   60 * time.Second,
	}

	return &PortalClient{http: client, jar: jar}, nil
}

// HTTPClient returns the underlying http.Client.
func (c *PortalClient) HTTPClient() *http.Client {
	return c.http
}

// CookieJar returns the cookie jar used by this client.
func (c *PortalClient) CookieJar() *cookiejar.Jar {
	return c.jar
}

// readAndLog reads resp.Body fully and, when Debug is true, prints the step,
// final URL, status code, and first 4 KiB of the body to stderr.
// The caller must not read resp.Body after calling this.
func (c *PortalClient) readAndLog(step string, resp *http.Response) ([]byte, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}
	if c.Debug {
		const maxLen = 4096
		snippet := body
		if len(body) > maxLen {
			snippet = append(body[:maxLen:maxLen], []byte("\n... (truncated)")...)
		}
		finalURL := ""
		if resp.Request != nil {
			finalURL = resp.Request.URL.String()
		}
		fmt.Fprintf(os.Stderr,
			"\n[DEBUG] step=%s  status=%d\n  url=%s\n%s\n",
			step, resp.StatusCode, finalURL, bytes.TrimSpace(snippet),
		)
	}
	return body, nil
}
