package portal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// ErrSessionExpired is returned when the portal responds with an HTML page instead of JSON,
// which indicates that the session has expired and SAML re-authentication is required.
var ErrSessionExpired = errors.New("portal session expired (got HTML, re-authentication required)")

const (
	V1Environments = "https://portal.commerce.ondemand.com/v1/subscriptions/%s/environments"
	V1Environment  = "https://portal.commerce.ondemand.com/v1/subscriptions/%s/environments/%s"
	V1Services     = "https://portal.commerce.ondemand.com/v1/subscriptions/%s/environments/%s/services"
)

// EndpointInfo represents a named HTTP endpoint of an environment.
type EndpointInfo struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	DomainName string `json:"domainName"`
	Protocol   string `json:"protocol"`
}

// EnvironmentInfo represents a single CCV2 environment from the API.
type EnvironmentInfo struct {
	Code      string         `json:"code"`
	Name      string         `json:"name"`
	Type      string         `json:"environmentType"`
	Status    string         `json:"status"`
	Endpoints []EndpointInfo `json:"endpoints"`
}

// ReplicaInfo represents an individual service replica (cluster node).
// The Name field is used as the value of the ROUTE cookie to target a specific node.
type ReplicaInfo struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Ready  bool   `json:"ready"`
}

// ServiceInfo represents a service within an environment.
type ServiceInfo struct {
	Code            string        `json:"code"`
	Name            string        `json:"name"`
	DomainName      string        `json:"domainName,omitempty"`
	ContextPath     string        `json:"contextPath,omitempty"`
	InitialPassword string        `json:"initialPassword,omitempty"`
	Replicas        []ReplicaInfo `json:"replicas,omitempty"`
}

// GetEnvironments returns all environments for the given subscription.
func (c *PortalClient) GetEnvironments(subscriptionCode string) ([]EnvironmentInfo, error) {
	u := fmt.Sprintf(V1Environments, subscriptionCode)
	resp, err := c.apiGet(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result []EnvironmentInfo
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding environments response: %w", err)
	}
	return result, nil
}

// GetEnvironment returns details for a single environment.
func (c *PortalClient) GetEnvironment(subscriptionCode, envCode string) (*EnvironmentInfo, error) {
	u := fmt.Sprintf(V1Environment, subscriptionCode, envCode)
	resp, err := c.apiGet(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var env EnvironmentInfo
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, fmt.Errorf("decoding environment response: %w", err)
	}
	return &env, nil
}

// GetServices returns services for a specific environment.
func (c *PortalClient) GetServices(subscriptionCode, envCode string) ([]ServiceInfo, error) {
	u := fmt.Sprintf(V1Services, subscriptionCode, envCode)
	resp, err := c.apiGet(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result []ServiceInfo
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding services response: %w", err)
	}
	return result, nil
}

// apiGet performs a GET request with appropriate headers.
func (c *PortalClient) apiGet(rawURL string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building GET request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)

	if c.Debug {
		fmt.Fprintf(os.Stderr, "\n[DEBUG] GET %s\n", rawURL)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", rawURL, err)
	}

	if c.Debug {
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("reading api_get body: %w", readErr)
		}
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
			"\n[DEBUG] api_get  status=%d  Content-Type=%s\n  url=%s\n%s\n",
			resp.StatusCode, resp.Header.Get("Content-Type"), finalURL, bytes.TrimSpace(snippet),
		)
		resp.Body = io.NopCloser(bytes.NewReader(body))
	}

	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, fmt.Errorf("GET %s returned HTTP %d", rawURL, resp.StatusCode)
	}
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
		resp.Body.Close()
		return nil, ErrSessionExpired
	}
	return resp, nil
}
