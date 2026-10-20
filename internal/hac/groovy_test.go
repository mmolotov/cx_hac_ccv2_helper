package hac

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExecuteScript(t *testing.T) {
	for _, commit := range []bool{false, true} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/console/scripting/execute" {
				t.Errorf("unexpected path %q", r.URL.Path)
			}
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if got := r.FormValue("script"); got != "return 42" {
				t.Errorf("script = %q", got)
			}
			if got := r.FormValue("scriptType"); got != "groovy" {
				t.Errorf("scriptType = %q", got)
			}
			want := "false"
			if commit {
				want = "true"
			}
			if got := r.FormValue("commit"); got != want {
				t.Errorf("commit = %q, want %q", got, want)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"executionResult":"42","outputText":"hi\n","stacktraceText":""}`))
		}))

		c := newTestClient(srv.URL)
		res, err := c.ExecuteScript(ScriptRequest{Script: "return 42", Commit: commit})
		srv.Close()
		if err != nil {
			t.Fatalf("ExecuteScript(commit=%v): %v", commit, err)
		}
		if res.ExecutionResult != "42" || res.OutputText != "hi\n" || res.HasError() {
			t.Errorf("unexpected result %+v", res)
		}
	}
}

func TestExecuteScriptStacktrace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"stacktraceText":"groovy.lang.MissingPropertyException: foo"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	res, err := c.ExecuteScript(ScriptRequest{Script: "foo"})
	if err != nil {
		t.Fatalf("ExecuteScript: %v", err)
	}
	if !res.HasError() {
		t.Error("expected HasError() to be true")
	}
}

func TestExecuteScriptErrors(t *testing.T) {
	cases := map[string]http.HandlerFunc{
		"http 500": func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		},
		"html": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html;charset=UTF-8")
			_, _ = w.Write([]byte("<html>login</html>"))
		},
	}
	for name, h := range cases {
		srv := httptest.NewServer(h)
		c := newTestClient(srv.URL)
		_, err := c.ExecuteScript(ScriptRequest{Script: "1"})
		srv.Close()
		if err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}
