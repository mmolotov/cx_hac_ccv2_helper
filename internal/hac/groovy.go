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

// ScriptRequest holds parameters for a Groovy script execution.
type ScriptRequest struct {
	Script string
	Commit bool
}

// ScriptResult holds the parsed HAC scripting response.
type ScriptResult struct {
	ExecutionResult string `json:"executionResult"`
	OutputText      string `json:"outputText"`
	StacktraceText  string `json:"stacktraceText"`
}

// HasError reports whether the script execution produced an exception.
func (r *ScriptResult) HasError() bool {
	return r.StacktraceText != ""
}

// ExecuteScript runs a Groovy script via the HAC scripting console.
func (c *Client) ExecuteScript(req ScriptRequest) (*ScriptResult, error) {
	form := url.Values{}
	form.Set("script", req.Script)
	form.Set("scriptType", "groovy")
	form.Set("commit", "false")
	if req.Commit {
		form.Set("commit", "true")
	}
	if c.csrf != "" {
		form.Set("_csrf", c.csrf)
	}

	if c.Debug {
		fmt.Fprintf(os.Stderr, "\n[DEBUG] POST %s/console/scripting/execute\n", c.baseURL)
		fmt.Fprintf(os.Stderr, "  script(len=%d) scriptType=groovy commit=%v\n",
			len(req.Script), req.Commit)
	}

	httpReq, err := http.NewRequest(
		http.MethodPost,
		c.baseURL+"/console/scripting/execute",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("X-Requested-With", "XMLHttpRequest")
	if c.csrf != "" {
		httpReq.Header.Set("X-CSRF-TOKEN", c.csrf)
		httpReq.Header.Set("X-XSRF-TOKEN", c.csrf)
	}
	start := time.Now()
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("script request: %w", err)
	}
	defer resp.Body.Close()

	elapsed := time.Since(start)
	if c.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] script HTTP %d, Content-Type: %s, wall-clock: %s\n",
			resp.StatusCode, resp.Header.Get("Content-Type"), elapsed.Round(time.Millisecond))
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
		fmt.Fprintf(os.Stderr, "[DEBUG] script response: %s\n", snippet)
	}

	if resp.StatusCode != http.StatusOK {
		snippet := strings.TrimSpace(string(body))
		if len(snippet) > 300 {
			snippet = snippet[:300] + "..."
		}
		return nil, fmt.Errorf("HAC scripting returned HTTP %d: %s", resp.StatusCode, snippet)
	}
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
		return nil, fmt.Errorf("HAC scripting returned HTML (session expired or wrong endpoint)")
	}

	var result ScriptResult
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &result, nil
}
