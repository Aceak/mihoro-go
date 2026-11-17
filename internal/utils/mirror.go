package utils

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

// NormalizeMirror validates and normalizes a GitHub mirror base URL.
//
// Mirror values may arrive with shell quotes, trailing slashes or surrounding
// whitespace; all of those are removed, so quoted and unquoted forms are
// equivalent. An empty input means "no mirror" and returns an empty string.
// The result never ends with "/".
func NormalizeMirror(raw string) (string, error) {
	v, err := NormalizeURLInput(raw)
	if err != nil {
		return "", fmt.Errorf("mirror must start with http:// or https://, e.g. --mirror https://ghfast.top")
	}
	if v == "" {
		return "", nil
	}
	// The value is written into a systemd unit; reject whitespace and control
	// characters instead of letting them split the exec arguments.
	if strings.ContainsAny(v, " \t\r\n") {
		return "", fmt.Errorf("mirror URL must not contain whitespace")
	}
	u, err := url.Parse(v)
	if err != nil || u.Host == "" || !strings.Contains(u.Host, ".") {
		return "", fmt.Errorf("mirror must include a domain, e.g. --mirror https://ghfast.top")
	}
	return strings.TrimRight(v, "/"), nil
}

// SetMirrorEnv normalizes raw and exports it as the GitHub mirror used by
// downloads. Empty input leaves the environment untouched. The normalized
// value is returned so callers can store or print it.
func SetMirrorEnv(raw string) (string, error) {
	mirror, err := NormalizeMirror(raw)
	if err != nil {
		return "", err
	}
	if mirror != "" {
		_ = os.Setenv(GithubMirrorEnv, mirror)
	}
	return mirror, nil
}
