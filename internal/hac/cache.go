package hac

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// ClearRegionCache clears all region caches via POST /monitoring/cache/regionCache/clear.
func (c *Client) ClearRegionCache() error {
	if c.Debug {
		fmt.Fprintf(os.Stderr, "\n[DEBUG] POST %s/monitoring/cache/regionCache/clear\n", c.baseURL)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/monitoring/cache/regionCache/clear", nil)
	if err != nil {
		return err
	}
	if c.csrf != "" {
		req.Header.Set("X-CSRF-TOKEN", c.csrf)
		req.Header.Set("X-XSRF-TOKEN", c.csrf)
	}

	start := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("cache clear request: %w", err)
	}
	defer resp.Body.Close()

	elapsed := time.Since(start)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if c.Debug {
		fmt.Fprintf(os.Stderr, "[DEBUG] cache/clear HTTP %d, wall-clock: %s\nresponse: %s\n",
			resp.StatusCode, elapsed.Round(time.Millisecond), body)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		snippet := strings.TrimSpace(string(body))
		if len(snippet) > 300 {
			snippet = snippet[:300] + "..."
		}
		return fmt.Errorf("HAC cache clear returned HTTP %d: %s", resp.StatusCode, snippet)
	}

	return nil
}
