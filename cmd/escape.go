package cmd

import (
	"fmt"
	"regexp"
	"strings"
)

// escapeSQLString escapes a value for use inside a single-quoted
// FlexibleSearch/SQL string literal by doubling single quotes.
func escapeSQLString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// groovyQuote returns s as a single-quoted Groovy string literal,
// escaping backslashes and single quotes.
func groovyQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return "'" + s + "'"
}

var (
	typeCodeRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*[!*]?$`)
	localeRe   = regexp.MustCompile(`^[A-Za-z]{2,3}(_[A-Za-z0-9]+)*$`)
)

// validateTypeCode rejects anything that is not a plain type code
// (optionally suffixed with ! or * as allowed by FlexibleSearch).
func validateTypeCode(s string) error {
	if !typeCodeRe.MatchString(s) {
		return fmt.Errorf("invalid type code %q", s)
	}
	return nil
}

// validateLocale rejects anything that is not a locale ISO code such as en or de_CH.
func validateLocale(s string) error {
	if !localeRe.MatchString(s) {
		return fmt.Errorf("invalid locale %q", s)
	}
	return nil
}
