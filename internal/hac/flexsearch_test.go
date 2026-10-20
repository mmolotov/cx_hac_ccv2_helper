package hac

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExecuteFlexSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/console/flexsearch/execute" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if r.Header.Get("X-CSRF-TOKEN") != "test-csrf-token" {
			t.Errorf("X-CSRF-TOKEN not set correctly")
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if got := r.FormValue("flexibleSearchQuery"); got != "SELECT {PK} FROM {Product}" {
			t.Errorf("flexibleSearchQuery = %q", got)
		}
		if got := r.FormValue("maxCount"); got != "10" {
			t.Errorf("maxCount = %q", got)
		}
		if got := r.FormValue("locale"); got != "en" {
			t.Errorf("locale = %q", got)
		}
		if got := r.FormValue("commit"); got != "false" {
			t.Errorf("commit = %q", got)
		}
		if got := r.FormValue("_csrf"); got != "test-csrf-token" {
			t.Errorf("_csrf = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"resultCount":2,"headers":["PK"],"resultList":[["1"],["2"]]}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	res, err := c.ExecuteFlexSearch(QueryRequest{Query: "SELECT {PK} FROM {Product}", MaxCount: 10, Locale: "en"})
	if err != nil {
		t.Fatalf("ExecuteFlexSearch: %v", err)
	}
	if res.ResultCount != 2 || len(res.Results) != 2 || len(res.Headers) != 1 || res.Headers[0] != "PK" {
		t.Errorf("unexpected result %+v", res)
	}
}

func TestExecuteSQL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if got := r.FormValue("sqlQuery"); got != "SELECT 1" {
			t.Errorf("sqlQuery = %q", got)
		}
		if r.FormValue("flexibleSearchQuery") != "" {
			t.Error("flexibleSearchQuery must not be set for SQL")
		}
		_, _ = w.Write([]byte(`{"resultCount":1,"headers":["1"],"resultList":[[1]]}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	if _, err := c.ExecuteSQL(QueryRequest{Query: "SELECT 1", MaxCount: 1}); err != nil {
		t.Fatalf("ExecuteSQL: %v", err)
	}
}

func TestExecuteFlexSearchException(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"exceptionStackTrace":"FlexibleSearchException: unknown type Foo"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.ExecuteFlexSearch(QueryRequest{Query: "SELECT {PK} FROM {Foo}", MaxCount: 1})
	if err == nil || !strings.Contains(err.Error(), "unknown type Foo") {
		t.Fatalf("expected query error with stack trace, got %v", err)
	}
}

func TestExecuteFlexSearchHTMLResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body>login</body></html>`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	if _, err := c.ExecuteFlexSearch(QueryRequest{Query: "SELECT {PK} FROM {Product}", MaxCount: 1}); err == nil {
		t.Fatal("expected error on HTML response")
	}
}
