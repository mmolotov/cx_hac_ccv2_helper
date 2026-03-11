package hac

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClearRegionCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/monitoring/cache/regionCache/clear" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.Header.Get("X-CSRF-TOKEN") != "test-csrf-token" {
			t.Errorf("X-CSRF-TOKEN not set correctly")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	if err := c.ClearRegionCache(); err != nil {
		t.Fatalf("ClearRegionCache: %v", err)
	}
}

func TestClearRegionCacheError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	err := c.ClearRegionCache()
	if err == nil {
		t.Fatal("expected error on HTTP 500")
	}
}
