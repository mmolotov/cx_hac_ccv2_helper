package hac

import (
	"crypto/tls"
	"net/http"
	"net/http/cookiejar"

	"golang.org/x/net/publicsuffix"
)

// newTestClient creates a Client suitable for use in tests.
// The provided baseURL should point to an httptest.Server.
func newTestClient(baseURL string) *Client {
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	return &Client{
		http: &http.Client{
			Jar:       jar,
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, //nolint:gosec
		},
		baseURL: baseURL,
		csrf:    "test-csrf-token",
	}
}
