package utils

import (
	"fmt"
	"net/url"
	"strings"
)

// NormalizeProxy validates and normalizes a proxy URL.
//
// An empty input means "no proxy". A missing scheme is treated as http://,
// so "127.0.0.1:7890" and "http://127.0.0.1:7890" are equivalent.
func NormalizeProxy(raw string) (string, error) {
	v := StripQuotes(raw)
	if v == "" {
		return "", nil
	}
	if !strings.Contains(v, "://") {
		v = "http://" + v
	}
	u, err := url.Parse(v)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("invalid proxy URL %q (expected e.g. http://127.0.0.1:7890)", raw)
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "socks5", "socks5h":
		// supported schemes
	default:
		return "", fmt.Errorf("unsupported proxy scheme %q (use http, https, socks5 or socks5h)", u.Scheme)
	}
	return u.String(), nil
}

// RedactProxy hides the password in a proxy URL for display.
// It returns the input unchanged if it cannot be parsed.
func RedactProxy(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return u.Redacted()
}
