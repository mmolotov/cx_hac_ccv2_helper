package hac

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/publicsuffix"
)

// Client is an authenticated HAC HTTP client.
type Client struct {
	http    *http.Client
	baseURL string
	csrf    string
	Debug   bool
}

// routeTransport injects a ROUTE cookie into every request so that all traffic
// (login, CSRF refresh, script execution) is pinned to the same cluster node.
type routeTransport struct {
	base  http.RoundTripper
	route string // full cookie value, e.g. ".api-8477ccd75c-jwxwn"
}

func (t *routeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r2 := req.Clone(req.Context())
	r2.AddCookie(&http.Cookie{Name: "ROUTE", Value: t.route})
	return t.base.RoundTrip(r2)
}

// SetRoute pins all subsequent requests to a specific cluster node.
// The cookie value is set to ".<route>" as expected by the load balancer.
func (c *Client) SetRoute(route string) {
	c.http.Transport = &routeTransport{
		base:  c.http.Transport,
		route: "." + route,
	}
}

// NewClient creates a HAC client targeting the given base URL
// (e.g. "https://backoffice.xxx.com/hac").
func NewClient(baseURL string) (*Client, error) {
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, err
	}
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		Proxy:           http.ProxyFromEnvironment,
	}
	httpClient := &http.Client{
		Transport: transport,
		Jar:       jar,
		Timeout:   300 * time.Second,
	}
	return &Client{
		http:    httpClient,
		baseURL: strings.TrimRight(baseURL, "/"),
	}, nil
}

// Login authenticates with HAC using Spring Security form login.
func (c *Client) Login(username, password string) error {
	// Fetch login page to obtain CSRF token.
	resp, err := c.http.Get(c.baseURL + "/login")
	if err != nil {
		return fmt.Errorf("fetching login page: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return fmt.Errorf("parsing login page: %w", err)
	}

	csrf, _ := doc.Find("input[name='_csrf']").Attr("value")
	if csrf == "" {
		csrf, _ = doc.Find("meta[name='_csrf']").Attr("content")
	}
	// Some HAC setups use CookieCsrfTokenRepository: token arrives as XSRF-TOKEN cookie.
	if csrf == "" {
		csrf = c.xsrfCookie()
	}
	if c.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] HAC login CSRF: %q\n", csrf)
	}

	form := url.Values{}
	form.Set("j_username", username)
	form.Set("j_password", password)
	if csrf != "" {
		form.Set("_csrf", csrf)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/j_spring_security_check", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if csrf != "" {
		req.Header.Set("X-CSRF-TOKEN", csrf)
		req.Header.Set("X-XSRF-TOKEN", csrf)
	}

	loginResp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("login POST: %w", err)
	}
	defer loginResp.Body.Close()
	io.Copy(io.Discard, loginResp.Body) //nolint:errcheck

	// Spring Security redirects to /login?error on bad credentials.
	if strings.Contains(loginResp.Request.URL.RawQuery, "error") {
		return fmt.Errorf("HAC authentication failed: invalid credentials")
	}

	return c.refreshCSRF()
}

// refreshCSRF fetches the HAC home page and updates the stored CSRF token.
// It checks both the HTML (session-based) and the XSRF-TOKEN cookie
// (CookieCsrfTokenRepository).
func (c *Client) refreshCSRF() error {
	resp, err := c.http.Get(c.baseURL + "/")
	if err != nil {
		return fmt.Errorf("refreshing CSRF: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return err
	}

	csrf, _ := doc.Find("meta[name='_csrf']").Attr("content")
	if csrf == "" {
		csrf, _ = doc.Find("input[name='_csrf']").Attr("value")
	}
	// Fall back to cookie-based CSRF (Spring CookieCsrfTokenRepository).
	if csrf == "" {
		csrf = c.xsrfCookie()
	}
	c.csrf = csrf

	if c.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] HAC CSRF (post-login): %q\n", csrf)
	}
	return nil
}

// xsrfCookie returns the value of the XSRF-TOKEN cookie from the jar, or "".
func (c *Client) xsrfCookie() string {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return ""
	}
	for _, cookie := range c.http.Jar.Cookies(u) {
		if cookie.Name == "XSRF-TOKEN" {
			return cookie.Value
		}
	}
	return ""
}
