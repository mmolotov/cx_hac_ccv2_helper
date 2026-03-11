package portal

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/mmolotov/cx_hac_ccv2_helper/internal/ui"
)

const (
	portalBase       = "https://portal.commerce.ondemand.com"
	subscriptionPath = "/subscription/%s/environments"
	userAgent        = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
)

// AuthConfig holds the credentials needed for portal authentication.
type AuthConfig struct {
	SAPLogin    string
	SAPPassword string
}

// Authenticate performs the IAS OAuth2 SSO flow for the given subscription.
//
// Flow:
//  1. GET portal → JS redirect → IAS login page (ycloud.accounts.ondemand.com)
//  2. POST username → intermediate form
//  3. POST intermediate form → password form  (same URL as step 2)
//  4. POST credentials → SAML ACS
//  5. POST ACS → 2FA choice → TOTP entry → OTP → portal/login/callback → session
func (c *PortalClient) Authenticate(subscriptionCode string, creds AuthConfig) error {
	startURL := fmt.Sprintf(portalBase+subscriptionPath, subscriptionCode)

	// Step 1: follow portal JS redirect → IAS login page.
	loginURL, loginData, err := c.initSession(startURL)
	if err != nil {
		return fmt.Errorf("init_session: %w", err)
	}
	idpSSOEndpoint := loginData["idpSSOEndpoint"]
	xsrfProtection := loginData["xsrfProtection"]

	// Step 2: submit username → intermediate form.
	loginData["j_username"] = creds.SAPLogin
	nextURL, nextData, err := c.submitForm(loginURL, loginData)
	if err != nil {
		return fmt.Errorf("submit_username: %w", err)
	}

	// Step 3: POST intermediate form → password form.
	_, passwordData, err := c.submitForm(nextURL, nextData)
	if err != nil {
		return fmt.Errorf("submit_intermediate: %w", err)
	}

	// Step 4: submit credentials → SAML ACS.
	passwordData["j_username"] = creds.SAPLogin
	passwordData["j_password"] = creds.SAPPassword
	acsURL, acsData, err := c.submitForm(nextURL, passwordData)
	if err != nil {
		return fmt.Errorf("submit_password: %w", err)
	}

	// Step 5: handle ACS; performs 2FA, then follows OAuth2 callback.
	if err := c.handleACS(acsURL, acsData, idpSSOEndpoint, xsrfProtection); err != nil {
		return fmt.Errorf("handle_acs: %w", err)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Auth step helpers
// ---------------------------------------------------------------------------

// initSession GETs the portal page, follows the JS redirect to the IAS login
// page, and returns the login form URL and its input fields.
func (c *PortalClient) initSession(startURL string) (formURL string, data map[string]string, err error) {
	req, err := http.NewRequest(http.MethodGet, startURL, nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	body, err := c.readAndLog("init_session", resp)
	if err != nil {
		return "", nil, err
	}

	redirectURL, cookies, ok := parseJSRedirect(body, startURL)
	if !ok {
		return "", nil, fmt.Errorf("expected JS redirect page from portal, got unexpected response")
	}

	portalURL, _ := url.Parse(portalBase)
	c.jar.SetCookies(portalURL, cookies)

	req2, err := http.NewRequest(http.MethodGet, redirectURL, nil)
	if err != nil {
		return "", nil, err
	}
	req2.Header.Set("User-Agent", userAgent)
	resp2, err := c.http.Do(req2)
	if err != nil {
		return "", nil, err
	}
	defer resp2.Body.Close()

	body, err = c.readAndLog("init_session_login", resp2)
	if err != nil {
		return "", nil, err
	}

	// Use the final URL after redirects as the base for resolving relative form actions.
	pageURL := redirectURL
	if resp2.Request != nil {
		pageURL = resp2.Request.URL.String()
	}

	doc, err := parseHTML(bytes.NewReader(body))
	if err != nil {
		return "", nil, err
	}
	action, formData, err := parseForm(doc)
	if err != nil {
		return "", nil, fmt.Errorf("parsing login form: %w", err)
	}

	// Resolve relative actions (e.g. "/oauth2/authorize") against the page URL.
	action = resolveAction(doc, action, pageURL)
	return action, formData, nil
}

// handleACS POSTs the SAML ACS data and handles the 2FA challenge that follows.
// The final OTP submission redirects through portal/login/callback, establishing
// the portal session.
func (c *PortalClient) handleACS(acsURL string, data map[string]string, idpSSOEndpoint, xsrfProtection string) error {
	resp, err := c.postForm(acsURL, data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := c.readAndLog("handle_acs", resp)
	if err != nil {
		return err
	}
	doc, err := parseHTML(bytes.NewReader(body))
	if err != nil {
		return err
	}

	// 2FA method choice page → select TOTP.
	jsessionid := ""
	if doc.Find("div[data-page='ids-two_factor_choice/logon']").Length() > 0 {
		doc, jsessionid, err = c.twoFactorChoiceTOTP(doc, idpSSOEndpoint)
		if err != nil {
			return fmt.Errorf("two_factor_choice_totp: %w", err)
		}
	}

	// OTP entry form.
	if doc.Find("form#secondFactorForm").Length() > 0 {
		return c.twoFactorAuth(doc, idpSSOEndpoint, xsrfProtection, jsessionid)
	}

	return fmt.Errorf("unexpected ACS response: no 2FA form found")
}

// twoFactorChoiceTOTP selects TOTP as the 2FA method and returns the OTP
// entry form document plus the JSESSIONID cookie set in the response.
func (c *PortalClient) twoFactorChoiceTOTP(doc *goquery.Document, idpSSOEndpoint string) (*goquery.Document, string, error) {
	endpoint, payload, err := parseForm(doc)
	if err != nil {
		return nil, "", fmt.Errorf("parsing 2FA choice form: %w", err)
	}
	endpoint = resolveAction(doc, endpoint, idpSSOEndpoint)

	payload["tfaChoiceTOTP"] = "true"
	payload["tfaChoice"] = ""

	resp, err := c.postForm(endpoint, payload)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	var jsessionid string
	for _, ck := range resp.Cookies() {
		if ck.Name == "JSESSIONID" {
			jsessionid = ck.Value
			break
		}
	}

	body, err := c.readAndLog("two_factor_choice_totp", resp)
	if err != nil {
		return nil, "", err
	}
	html, err := parseHTML(bytes.NewReader(body))
	if err != nil {
		return nil, "", err
	}
	return html, jsessionid, nil
}

// twoFactorAuth prompts for the TOTP code, submits it, and confirms that the
// http.Client was redirected to the portal (via /login/callback), which means
// the session has been established.
func (c *PortalClient) twoFactorAuth(doc *goquery.Document, idpSSOEndpoint, xsrfProtection, jsessionid string) error {
	form := doc.Find("form#secondFactorForm")
	if form.Length() == 0 {
		form = doc.Find("form").First()
	}
	rawAction, _ := form.Attr("action")
	endpoint := resolveAction(doc, rawAction, idpSSOEndpoint)

	payload := make(map[string]string)
	form.Find("input").Each(func(_ int, s *goquery.Selection) {
		name, ok := s.Attr("name")
		if !ok || name == "" {
			return
		}
		val, _ := s.Attr("value")
		payload[name] = val
	})

	otp, err := ui.PromptOTP("Enter SAP 2FA TOTP code")
	if err != nil {
		return err
	}
	payload["j_otpcode"] = otp
	payload["continue"] = "tfaTOTP"

	// SAP IAS requires xsrfProtection both in the form body and as an HTTP header.
	headers := map[string]string{
		"X-CSRF-Token": xsrfProtection,
		"X-XSRF-TOKEN": xsrfProtection,
	}

	resp, err := c.postFormEx(endpoint, payload, []*http.Cookie{{Name: "JSESSIONID", Value: jsessionid}}, headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := c.readAndLog("two_factor_auth", resp)
	if err != nil {
		return err
	}

	// The http.Client automatically follows: OTP POST → 302 → /login/callback → portal home.
	finalURL := endpoint
	if resp.Request != nil {
		finalURL = resp.Request.URL.String()
	}
	if strings.HasPrefix(finalURL, portalBase) {
		return nil
	}

	// Fallback: detect a JS redirect page (loading animation) and follow it.
	if redirectURL, _, ok := parseJSRedirect(body, endpoint); ok {
		req, err := http.NewRequest(http.MethodGet, redirectURL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", userAgent)
		resp2, err := c.http.Do(req)
		if err != nil {
			return err
		}
		defer resp2.Body.Close()
		if _, err := c.readAndLog("two_factor_auth_callback", resp2); err != nil {
			return err
		}
		finalURL = redirectURL
		if resp2.Request != nil {
			finalURL = resp2.Request.URL.String()
		}
		if strings.HasPrefix(finalURL, portalBase) {
			return nil
		}
	}

	return fmt.Errorf("OTP submission did not redirect to portal (landed on: %s)", finalURL)
}

// ---------------------------------------------------------------------------
// HTML helpers
// ---------------------------------------------------------------------------

// parseJSRedirect detects the JavaScript redirect page the portal emits and
// extracts the redirect URL plus the cookies that a real browser would set.
// Returns (redirectURL, cookies, true) when detected, ("", nil, false) otherwise.
func parseJSRedirect(body []byte, startURL string) (redirectURL string, cookies []*http.Cookie, ok bool) {
	s := string(body)
	if !strings.Contains(s, "document.cookie=") || !strings.Contains(s, `location="`) {
		return "", nil, false
	}

	locIdx := strings.Index(s, `location="`)
	if locIdx < 0 {
		return "", nil, false
	}
	locStart := locIdx + len(`location="`)
	locEnd := strings.Index(s[locStart:], `"`)
	if locEnd < 0 {
		return "", nil, false
	}
	redirectURL = s[locStart : locStart+locEnd]

	// Extract signature cookie (already percent-encoded in the script).
	var signature string
	if i := strings.Index(s, `"signature=`); i >= 0 {
		vStart := i + len(`"signature=`)
		if j := strings.Index(s[vStart:], ";"); j >= 0 {
			signature = s[vStart : vStart+j]
		}
	}

	// Replicate browser: locationAfterLogin = encodeURIComponent(path+query of startURL).
	var locationAfterLogin string
	if parsed, err := url.Parse(startURL); err == nil {
		locationAfterLogin = url.QueryEscape(parsed.RequestURI())
	}

	cookies = []*http.Cookie{
		{Name: "fragmentAfterLogin", Value: "", Path: "/"},
		{Name: "locationAfterLogin", Value: locationAfterLogin, Path: "/"},
		{Name: "signature", Value: signature, Path: "/"},
	}
	return redirectURL, cookies, true
}

// resolveAction returns action as-is if absolute, otherwise resolves it against
// the document's <base href> tag. Falls back to fallbackBase if no base tag is found.
func resolveAction(doc *goquery.Document, action, fallbackBase string) string {
	if strings.HasPrefix(action, "https://") || strings.HasPrefix(action, "http://") {
		return action
	}
	base, _ := doc.Find("base[href]").Attr("href")
	if base == "" {
		base = fallbackBase
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return action
	}
	ref, err := url.Parse(action)
	if err != nil {
		return action
	}
	return baseURL.ResolveReference(ref).String()
}

func parseHTML(r io.Reader) (*goquery.Document, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, fmt.Errorf("parsing HTML: %w", err)
	}
	return doc, nil
}

// parseForm extracts the action URL and all input[name]→value pairs from the
// first <form> in the document.
func parseForm(doc *goquery.Document) (action string, data map[string]string, err error) {
	form := doc.Find("form").First()
	if form.Length() == 0 {
		return "", nil, fmt.Errorf("no <form> found in response")
	}
	action, _ = form.Attr("action")
	data = make(map[string]string)
	form.Find("input").Each(func(_ int, s *goquery.Selection) {
		name, hasName := s.Attr("name")
		if !hasName || name == "" {
			return
		}
		val, _ := s.Attr("value")
		data[name] = val
	})
	return action, data, nil
}

// parseFormInput extracts a single input value by name from anywhere in the doc.
func parseFormInput(doc *goquery.Document, name string) (string, error) {
	val, exists := doc.Find(fmt.Sprintf("input[name=%q]", name)).Attr("value")
	if !exists {
		return "", fmt.Errorf("input[name=%s] not found", name)
	}
	return val, nil
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

// submitForm POSTs form data and parses the first <form> from the response.
func (c *PortalClient) submitForm(rawURL string, data map[string]string) (action string, out map[string]string, err error) {
	resp, err := c.postForm(rawURL, data)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	body, err := c.readAndLog("submit_form", resp)
	if err != nil {
		return "", nil, err
	}
	doc, err := parseHTML(bytes.NewReader(body))
	if err != nil {
		return "", nil, err
	}
	action, out, err = parseForm(doc)
	if err != nil {
		return "", nil, fmt.Errorf("parsing response form: %w", err)
	}
	return action, out, nil
}

// postForm sends a POST with application/x-www-form-urlencoded body.
func (c *PortalClient) postForm(rawURL string, data map[string]string) (*http.Response, error) {
	return c.postFormEx(rawURL, data, nil, nil)
}

// postFormEx is the underlying POST helper; cookies and extraHeaders are optional.
func (c *PortalClient) postFormEx(rawURL string, data map[string]string, cookies []*http.Cookie, extraHeaders map[string]string) (*http.Response, error) {
	form := url.Values{}
	for k, v := range data {
		form.Set(k, v)
	}

	if c.Debug {
		keys := make([]string, 0, len(data))
		for k := range data {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		fmt.Fprintf(os.Stderr, "\n[DEBUG] POST %s\n", rawURL)
		for _, k := range keys {
			v := data[k]
			if k == "j_password" || k == "j_otpcode" {
				v = "***"
			}
			fmt.Fprintf(os.Stderr, "  %s=%s\n", k, v)
		}
	}

	req, err := http.NewRequest(http.MethodPost, rawURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("building POST request to %s: %w", rawURL, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("User-Agent", userAgent)
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", rawURL, err)
	}
	return resp, nil
}
