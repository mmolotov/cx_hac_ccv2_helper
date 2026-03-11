package portal

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/net/publicsuffix"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newTestPortalClient creates a PortalClient backed by the provided *http.Client.
// Tests in the same package can access unexported fields directly.
func newTestPortalClient(hc *http.Client, jar *cookiejar.Jar) *PortalClient {
	return &PortalClient{http: hc, jar: jar}
}

func newJar(t *testing.T) *cookiejar.Jar {
	t.Helper()
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		t.Fatal(err)
	}
	return jar
}

// ---------------------------------------------------------------------------
// parseJSRedirect
// ---------------------------------------------------------------------------

func TestParseJSRedirectDetected(t *testing.T) {
	body := []byte(`<html><head><script nonce="x">
document.cookie="fragmentAfterLogin="+encodeURIComponent(location.hash)+";path=/;";
document.cookie="locationAfterLogin="+encodeURIComponent(location.href.split('#')[0].split(location.host)[1])+";path=/;";
document.cookie="signature=ABC%3D%3D;path=/;";
location="https://accounts.example.com/oauth2/authorize?client_id=xyz"
</script></head></html>`)

	redirectURL, cookies, ok := parseJSRedirect(body, "https://portal.example.com/subscription/sub1/environments")

	if !ok {
		t.Fatal("expected JS redirect to be detected")
	}
	if redirectURL != "https://accounts.example.com/oauth2/authorize?client_id=xyz" {
		t.Errorf("redirectURL = %q", redirectURL)
	}

	byName := make(map[string]string)
	for _, c := range cookies {
		byName[c.Name] = c.Value
	}

	if byName["signature"] != "ABC%3D%3D" {
		t.Errorf("signature cookie = %q, want %q", byName["signature"], "ABC%3D%3D")
	}
	if byName["fragmentAfterLogin"] != "" {
		t.Errorf("fragmentAfterLogin should be empty, got %q", byName["fragmentAfterLogin"])
	}
	// locationAfterLogin = encodeURIComponent("/subscription/sub1/environments")
	want := "%2Fsubscription%2Fsub1%2Fenvironments"
	if byName["locationAfterLogin"] != want {
		t.Errorf("locationAfterLogin = %q, want %q", byName["locationAfterLogin"], want)
	}
}

func TestParseJSRedirectNotDetected(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{
			name: "no document.cookie",
			body: `<html><script>location="https://example.com"</script></html>`,
		},
		{
			name: "no location=",
			body: `<html><script>document.cookie="x=1";</script></html>`,
		},
		{
			name: "regular login page",
			body: `<html><body><form action="/login"><input name="j_username"/></form></body></html>`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, ok := parseJSRedirect([]byte(tc.body), "https://portal.example.com/")
			if ok {
				t.Fatal("expected no detection")
			}
		})
	}
}

func TestParseJSRedirectNoSignature(t *testing.T) {
	body := []byte(`<script>document.cookie="x=1;path=/;";location="https://example.com/auth"</script>`)
	redirectURL, cookies, ok := parseJSRedirect(body, "https://portal.example.com/")
	if !ok {
		t.Fatal("expected detection")
	}
	if redirectURL != "https://example.com/auth" {
		t.Errorf("redirectURL = %q", redirectURL)
	}
	for _, c := range cookies {
		if c.Name == "signature" && c.Value != "" {
			t.Errorf("expected empty signature, got %q", c.Value)
		}
	}
}

// ---------------------------------------------------------------------------
// resolveAction
// ---------------------------------------------------------------------------

func TestResolveActionAbsolute(t *testing.T) {
	doc, _ := parseHTML(strings.NewReader(`<html><body></body></html>`))
	got := resolveAction(doc, "https://accounts.example.com/sso", "https://fallback.example.com")
	if got != "https://accounts.example.com/sso" {
		t.Errorf("got %q", got)
	}
}

func TestResolveActionRelativeWithBaseHref(t *testing.T) {
	html := `<html><head><base href="https://accounts.example.com/idp/"></head><body></body></html>`
	doc, _ := parseHTML(strings.NewReader(html))
	got := resolveAction(doc, "/oauth2/authorize", "https://fallback.example.com")
	if got != "https://accounts.example.com/oauth2/authorize" {
		t.Errorf("got %q", got)
	}
}

func TestResolveActionRelativeWithFallback(t *testing.T) {
	doc, _ := parseHTML(strings.NewReader(`<html><body></body></html>`))
	got := resolveAction(doc, "/oauth2/authorize", "https://accounts.example.com/some/path")
	if got != "https://accounts.example.com/oauth2/authorize" {
		t.Errorf("got %q", got)
	}
}

func TestResolveActionHTTP(t *testing.T) {
	doc, _ := parseHTML(strings.NewReader(`<html><body></body></html>`))
	got := resolveAction(doc, "http://example.com/path", "https://fallback.example.com")
	if got != "http://example.com/path" {
		t.Errorf("got %q", got)
	}
}

// ---------------------------------------------------------------------------
// submitForm
// ---------------------------------------------------------------------------

func TestSubmitFormParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(w, `<html><body>
<form action="https://next.example.com/step2">
  <input type="hidden" name="token" value="tok-%s"/>
</form></body></html>`, r.FormValue("id"))
	}))
	defer srv.Close()

	jar := newJar(t)
	pc := newTestPortalClient(&http.Client{Jar: jar}, jar)

	action, data, err := pc.submitForm(srv.URL+"/step1", map[string]string{"id": "abc"})
	if err != nil {
		t.Fatalf("submitForm: %v", err)
	}
	if action != "https://next.example.com/step2" {
		t.Errorf("action = %q", action)
	}
	if data["token"] != "tok-abc" {
		t.Errorf("token = %q", data["token"])
	}
}

func TestSubmitFormNoFormInResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><body><p>No form here</p></body></html>`)
	}))
	defer srv.Close()

	jar := newJar(t)
	pc := newTestPortalClient(&http.Client{Jar: jar}, jar)

	_, _, err := pc.submitForm(srv.URL+"/step", map[string]string{})
	if err == nil {
		t.Fatal("expected error when response has no form")
	}
}

// ---------------------------------------------------------------------------
// initSession
// ---------------------------------------------------------------------------

func TestInitSessionFollowsJSRedirect(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Portal page: returns the JS redirect body pointing to the test server's login page.
	mux.HandleFunc("/subscription/sub1/environments", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `<html><head><script>
document.cookie="signature=SIG;path=/;";
document.cookie="fragmentAfterLogin=;path=/;";
document.cookie="locationAfterLogin=%s;path=/;";
location="%s/login"
</script></head></html>`, "%2Fsubscription%2Fsub1%2Fenvironments", srv.URL)
	})

	// IAS login page: the login form returned after JS redirect.
	mux.HandleFunc("/login", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `<html><body>
<form action="/oauth2/authorize">
  <input type="hidden" name="idpSSOEndpoint" value="https://accounts.example.com/sso"/>
  <input type="hidden" name="xsrfProtection" value="csrf-token-42"/>
  <input type="text" name="j_username"/>
</form></body></html>`)
	})

	jar := newJar(t)
	pc := newTestPortalClient(&http.Client{Jar: jar}, jar)

	formURL, data, err := pc.initSession(srv.URL + "/subscription/sub1/environments")
	if err != nil {
		t.Fatalf("initSession: %v", err)
	}

	// Action "/oauth2/authorize" must be resolved against the login page URL.
	wantAction := srv.URL + "/oauth2/authorize"
	if formURL != wantAction {
		t.Errorf("formURL = %q, want %q", formURL, wantAction)
	}
	if data["idpSSOEndpoint"] != "https://accounts.example.com/sso" {
		t.Errorf("idpSSOEndpoint = %q", data["idpSSOEndpoint"])
	}
	if data["xsrfProtection"] != "csrf-token-42" {
		t.Errorf("xsrfProtection = %q", data["xsrfProtection"])
	}
}

func TestInitSessionNoJSRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<html><body><p>Unexpected response</p></body></html>`)
	}))
	defer srv.Close()

	jar := newJar(t)
	pc := newTestPortalClient(&http.Client{Jar: jar}, jar)

	_, _, err := pc.initSession(srv.URL + "/subscription/sub1/environments")
	if err == nil {
		t.Fatal("expected error when portal does not return JS redirect")
	}
}

// ---------------------------------------------------------------------------
// parseForm (existing, kept for completeness)
// ---------------------------------------------------------------------------

func TestParseForm(t *testing.T) {
	html := `<!DOCTYPE html><html><body>
<form action="https://example.com/sso" method="post">
  <input type="hidden" name="SAMLRequest" value="abc123" />
  <input type="hidden" name="RelayState" value="relay-xyz" />
  <input type="submit" value="Submit" />
</form>
</body></html>`

	doc, err := parseHTML(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parseHTML: %v", err)
	}
	action, data, err := parseForm(doc)
	if err != nil {
		t.Fatalf("parseForm: %v", err)
	}
	if action != "https://example.com/sso" {
		t.Errorf("action: got %q, want %q", action, "https://example.com/sso")
	}
	if data["SAMLRequest"] != "abc123" {
		t.Errorf("SAMLRequest: got %q", data["SAMLRequest"])
	}
	if data["RelayState"] != "relay-xyz" {
		t.Errorf("RelayState: got %q", data["RelayState"])
	}
}

func TestParseFormNoForm(t *testing.T) {
	html := `<html><body><p>No form here</p></body></html>`
	doc, err := parseHTML(strings.NewReader(html))
	if err != nil {
		t.Fatalf("parseHTML: %v", err)
	}
	_, _, err = parseForm(doc)
	if err == nil {
		t.Fatal("expected error when no form present")
	}
}

func TestParseFormInput(t *testing.T) {
	html := `<html><body><input name="xsrfProtection" value="tok-42" /></body></html>`
	doc, _ := parseHTML(strings.NewReader(html))

	val, err := parseFormInput(doc, "xsrfProtection")
	if err != nil {
		t.Fatalf("parseFormInput: %v", err)
	}
	if val != "tok-42" {
		t.Errorf("got %q, want %q", val, "tok-42")
	}
}

func TestParseFormInputMissing(t *testing.T) {
	doc, _ := parseHTML(strings.NewReader(`<html><body></body></html>`))
	_, err := parseFormInput(doc, "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing input")
	}
}

func TestDetect2FA(t *testing.T) {
	html := `<html><body>
<div data-page="ids-two_factor_choice/logon">
  <input name="tfaToken" value="my-token" />
</div>
</body></html>`
	doc, _ := parseHTML(strings.NewReader(html))

	if doc.Find("div[data-page='ids-two_factor_choice/logon']").Length() == 0 {
		t.Fatal("expected to detect 2FA choice page")
	}
	tfaToken, exists := doc.Find("input[name='tfaToken']").Attr("value")
	if !exists || tfaToken != "my-token" {
		t.Errorf("tfaToken: got %q, want %q", tfaToken, "my-token")
	}
}
