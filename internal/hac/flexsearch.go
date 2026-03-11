package hac

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// QueryRequest holds parameters for a FlexibleSearch or SQL query.
type QueryRequest struct {
	Query              string
	MaxCount           int
	Locale             string
	QueryTimeout       int // milliseconds; 0 = disabled
	CancelQueryTimeout int // milliseconds; 0 = disabled
}

// QueryResult holds the parsed HAC query response.
type QueryResult struct {
	ResultCount         int             `json:"resultCount"`
	ExecutionTime       int             `json:"executionTime"` // ms, server-reported
	Headers             []string        `json:"headers"`
	Results             [][]interface{} `json:"resultList"`
	ExceptionStackTrace string          `json:"exceptionStackTrace"`
}

// ExecuteFlexSearch runs a FlexibleSearch query.
func (c *Client) ExecuteFlexSearch(req QueryRequest) (*QueryResult, error) {
	return c.executeQuery("/console/flexsearch/execute", "flexibleSearchQuery", req)
}

// ExecuteSQL runs a direct SQL query.
func (c *Client) ExecuteSQL(req QueryRequest) (*QueryResult, error) {
	return c.executeQuery("/console/sql/execute", "sqlQuery", req)
}

func (c *Client) executeQuery(path, queryParam string, req QueryRequest) (*QueryResult, error) {
	form := url.Values{}
	form.Set(queryParam, req.Query)
	form.Set("maxCount", fmt.Sprintf("%d", req.MaxCount))
	form.Set("commit", "false")
	form.Set("user", "admin")
	if req.Locale != "" {
		form.Set("locale", req.Locale)
	}
	if req.QueryTimeout > 0 {
		form.Set("queryTimeout", fmt.Sprintf("%d", req.QueryTimeout))
	}
	if req.CancelQueryTimeout > 0 {
		form.Set("cancelQueryTimeout", fmt.Sprintf("%d", req.CancelQueryTimeout))
	}
	if c.csrf != "" {
		form.Set("_csrf", c.csrf)
	}

	encoded := form.Encode()

	if c.Debug {
		fmt.Fprintf(os.Stderr, "\n[DEBUG] POST %s%s\n", c.baseURL, path)
		for key, vals := range form {
			v := vals[0]
			if key == "_csrf" {
				v = "***"
			}
			fmt.Fprintf(os.Stderr, "  %s=%s\n", key, v)
		}
	}

	httpReq, err := http.NewRequest(http.MethodPost, c.baseURL+path, strings.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("Accept", "application/json")
	if c.csrf != "" {
		// Support both session-based (X-CSRF-TOKEN) and cookie-based (X-XSRF-TOKEN) CSRF.
		httpReq.Header.Set("X-CSRF-TOKEN", c.csrf)
		httpReq.Header.Set("X-XSRF-TOKEN", c.csrf)
	}

	start := time.Now()
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("query request: %w", err)
	}
	defer resp.Body.Close()

	elapsed := time.Since(start)
	if c.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] HAC query wall-clock time: %s\n", elapsed.Round(time.Millisecond))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if c.Debug {
		const maxLen = 4096
		snippet := body
		if len(body) > maxLen {
			snippet = append(body[:maxLen:maxLen], "...(truncated)"...)
		}
		fmt.Fprintf(os.Stderr, "[DEBUG] HAC response body: %s\n", snippet)
	}

	var result QueryResult
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	if result.ExceptionStackTrace != "" {
		return nil, fmt.Errorf("query error:\n%s", result.ExceptionStackTrace)
	}
	return &result, nil
}
