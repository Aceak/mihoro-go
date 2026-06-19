package mihoro

import (
	"testing"
)

func TestExtractMihomoVersionStable(t *testing.T) {
	output := "Mihomo Meta v1.19.23 linux amd64 with go1.25.1 2026-04-07"
	v := extractMihomoVersion(output)
	if v != "v1.19.23" {
		t.Errorf("got %q, want v1.19.23", v)
	}
}

func TestExtractMihomoVersionBare(t *testing.T) {
	output := "Mihomo Meta 1.19.23 linux amd64 with go1.25.1 2026-04-07"
	v := extractMihomoVersion(output)
	if v != "v1.19.23" {
		t.Errorf("got %q, want v1.19.23", v)
	}
}

func TestExtractMihomoVersionAlpha(t *testing.T) {
	output := "Mihomo Meta alpha-c107c6a linux amd64 with go1.25.1"
	v := extractMihomoVersion(output)
	if v != "alpha-c107c6a" {
		t.Errorf("got %q, want alpha-c107c6a", v)
	}
}

func TestExtractMihomoVersionEmpty(t *testing.T) {
	v := extractMihomoVersion("")
	if v != "" {
		t.Errorf("expected empty, got %q", v)
	}
}

func TestNormalizeVersionToken(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"v1.19.23", "v1.19.23"},
		{"v1.19.23,", "v1.19.23"},
		{"1.19.23", "v1.19.23"},
		{"alpha-c107c6a", "alpha-c107c6a"},
		{"linux", ""},
		{"go1.25.1", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := normalizeVersionToken(tt.input)
		if got != tt.want {
			t.Errorf("normalizeVersionToken(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractMihomoVersionFromStderr(t *testing.T) {
	// Version can appear in stderr too
	output := "Warning: some error\nMihomo Meta v2.0.0 linux amd64\n"
	v := extractMihomoVersion(output)
	if v != "v2.0.0" {
		t.Errorf("got %q, want v2.0.0", v)
	}
}

func TestExtractMihomoVersionNoVersion(t *testing.T) {
	output := "some random output without version"
	v := extractMihomoVersion(output)
	if v != "" {
		t.Errorf("expected empty, got %q", v)
	}
}

func TestExtractMihomoVersionBadFormat(t *testing.T) {
	// Something that looks like version but isn't
	v := extractMihomoVersion("go1.25.6")
	if v != "" {
		t.Errorf("go version should not match, got %q", v)
	}
}
