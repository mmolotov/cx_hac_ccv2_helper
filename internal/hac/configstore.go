package hac

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// SetConfigProperty writes a single property to the HAC platform config store
// via POST /platform/configstore with form body key=...&val=...
func (c *Client) SetConfigProperty(key, value string) error {
	form := url.Values{}
	form.Set("key", key)
	form.Set("val", value)
	if c.csrf != "" {
		form.Set("_csrf", c.csrf)
	}

	if c.Debug {
		fmt.Fprintf(os.Stderr, "\n[DEBUG] POST %s/platform/configstore key=%s\n", c.baseURL, key)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/platform/configstore", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if c.csrf != "" {
		req.Header.Set("X-CSRF-TOKEN", c.csrf)
		req.Header.Set("X-XSRF-TOKEN", c.csrf)
	}

	start := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("configstore request: %w", err)
	}
	defer resp.Body.Close()

	elapsed := time.Since(start)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if c.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] configstore HTTP %d, wall-clock: %s\nresponse: %s\n",
			resp.StatusCode, elapsed.Round(time.Millisecond), body)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		snippet := strings.TrimSpace(string(body))
		if len(snippet) > 300 {
			snippet = snippet[:300] + "..."
		}
		return fmt.Errorf("HAC configstore returned HTTP %d: %s", resp.StatusCode, snippet)
	}

	return nil
}
