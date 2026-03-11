package hac

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChangeLogLevel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/platform/log4j/changeLevel" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded; charset=UTF-8" {
			t.Errorf("Content-Type = %q", ct)
		}
		if r.Header.Get("X-CSRF-TOKEN") != "test-csrf-token" {
			t.Errorf("X-CSRF-TOKEN not set correctly")
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("loggerName") != "de.hybris.platform.order" {
			t.Errorf("loggerName = %q", r.FormValue("loggerName"))
		}
		if r.FormValue("levelName") != "DEBUG" {
			t.Errorf("levelName = %q", r.FormValue("levelName"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	if err := c.ChangeLogLevel("de.hybris.platform.order", "DEBUG"); err != nil {
		t.Fatalf("ChangeLogLevel: %v", err)
	}
}

func TestChangeLogLevelError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	err := c.ChangeLogLevel("some.logger", "INFO")
	if err == nil {
		t.Fatal("expected error on HTTP 403")
	}
}
