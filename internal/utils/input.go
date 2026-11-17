package utils

import (
	"fmt"
	"strings"
)

// StripQuotes removes surrounding whitespace and one pair of matching single
// or double quotes, as commonly pasted from config examples. Quoted and
// unquoted input become equivalent.
func StripQuotes(raw string) string {
	v := strings.TrimSpace(raw)
	return strings.TrimSpace(strings.Trim(v, `"'`))
}

// NormalizeURLInput cleans a user-provided URL (whitespace, surrounding
// quotes) and requires an http:// or https:// scheme. Empty input returns an
// empty string without error.
func NormalizeURLInput(raw string) (string, error) {
	v := StripQuotes(raw)
	if v == "" {
		return "", nil
	}
	if !strings.HasPrefix(v, "http://") && !strings.HasPrefix(v, "https://") {
		return "", fmt.Errorf("URL must start with http:// or https:// (got %q)", v)
	}
	return v, nil
}
