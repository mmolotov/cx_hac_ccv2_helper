package cmd

import "testing"

func TestEscapeSQLString(t *testing.T) {
	cases := map[string]string{
		"plain":   "plain",
		"o'brien": "o''brien",
		"''":      "''''",
		"":        "",
	}
	for in, want := range cases {
		if got := escapeSQLString(in); got != want {
			t.Errorf("escapeSQLString(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGroovyQuote(t *testing.T) {
	cases := map[string]string{
		"mail.smtp.server": `'mail.smtp.server'`,
		"it's":             `'it\'s'`,
		`a\b`:              `'a\\b'`,
		`x\'); evil(`:      `'x\\\'); evil('`,
	}
	for in, want := range cases {
		if got := groovyQuote(in); got != want {
			t.Errorf("groovyQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateTypeCode(t *testing.T) {
	for _, ok := range []string{"Product", "CronJob", "Product!", "Order*", "_x1"} {
		if err := validateTypeCode(ok); err != nil {
			t.Errorf("validateTypeCode(%q) unexpected error: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "1Product", "Product}", "Product} UNION {User", "a b"} {
		if err := validateTypeCode(bad); err == nil {
			t.Errorf("validateTypeCode(%q) expected error", bad)
		}
	}
}

func TestValidateLocale(t *testing.T) {
	for _, ok := range []string{"en", "de", "de_CH", "zh_Hant_TW"} {
		if err := validateLocale(ok); err != nil {
			t.Errorf("validateLocale(%q) unexpected error: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "e", "en]", "en] OR"} {
		if err := validateLocale(bad); err == nil {
			t.Errorf("validateLocale(%q) expected error", bad)
		}
	}
}
