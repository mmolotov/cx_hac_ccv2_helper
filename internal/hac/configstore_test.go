package hac

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSetConfigProperty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/platform/configstore" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.Header.Get("X-CSRF-TOKEN") != "test-csrf-token" {
			t.Errorf("X-CSRF-TOKEN not set correctly")
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.FormValue("key") != "my.prop" || r.FormValue("val") != "a=b c" {
			t.Errorf("key/val = %q/%q", r.FormValue("key"), r.FormValue("val"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	if err := c.SetConfigProperty("my.prop", "a=b c"); err != nil {
		t.Fatalf("SetConfigProperty: %v", err)
	}
}

func TestSetConfigPropertyError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	if err := c.SetConfigProperty("k", "v"); err == nil {
		t.Fatal("expected error on HTTP 403")
	}
}
