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

// ChangeLogLevel sets a Log4j logger to the given level via POST /platform/log4j/changeLevel.
func (c *Client) ChangeLogLevel(loggerName, levelName string) error {
	form := url.Values{}
	form.Set("loggerName", loggerName)
	form.Set("levelName", levelName)
	if c.csrf != "" {
		form.Set("_csrf", c.csrf)
	}

	if c.Debug {
		fmt.Fprintf(os.Stderr, "\n[DEBUG] POST %s/platform/log4j/changeLevel loggerName=%s levelName=%s\n",
			c.baseURL, loggerName, levelName)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/platform/log4j/changeLevel", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("Accept", "application/json")
	if c.csrf != "" {
		req.Header.Set("X-CSRF-TOKEN", c.csrf)
		req.Header.Set("X-XSRF-TOKEN", c.csrf)
	}

	start := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("loglevel request: %w", err)
	}
	defer resp.Body.Close()

	elapsed := time.Since(start)
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if c.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] log4j/changeLevel HTTP %d, wall-clock: %s\nresponse: %s\n",
			resp.StatusCode, elapsed.Round(time.Millisecond), respBody)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		snippet := strings.TrimSpace(string(respBody))
		if len(snippet) > 300 {
			snippet = snippet[:300] + "..."
		}
		return fmt.Errorf("HAC log4j/changeLevel returned HTTP %d: %s", resp.StatusCode, snippet)
	}

	return nil
}
