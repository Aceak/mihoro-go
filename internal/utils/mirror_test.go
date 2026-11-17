package utils

import (
	"os"
	"testing"
)

func TestNormalizeMirror(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"plain", "https://ghfast.top", "https://ghfast.top", false},
		{"trailing slash", "https://ghfast.top/", "https://ghfast.top", false},
		{"multiple trailing slashes", "https://ghfast.top///", "https://ghfast.top", false},
		{"double quotes", `"https://ghfast.top"`, "https://ghfast.top", false},
		{"single quotes", "'https://ghfast.top'", "https://ghfast.top", false},
		{"quotes with slash", `"https://ghfast.top/"`, "https://ghfast.top", false},
		{"surrounding spaces", "  https://ghfast.top  ", "https://ghfast.top", false},
		{"http scheme", "http://ghfast.top", "http://ghfast.top", false},
		{"with port", "https://ghfast.top:8080", "https://ghfast.top:8080", false},
		{"empty", "", "", false},
		{"blank", "   ", "", false},
		{"missing scheme", "ghfast.top", "", true},
		{"unsupported scheme", "ftp://ghfast.top", "", true},
		{"no domain", "https://ghfast", "", true},
		{"scheme only", "https://", "", true},
		{"inner space", "https://gh fast.top", "", true},
		{"newline injection", "https://ghfast.top\n--mirror evil", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeMirror(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NormalizeMirror(%q) = %q, want error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeMirror(%q) = %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("NormalizeMirror(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSetMirrorEnv(t *testing.T) {
	old := os.Getenv(GithubMirrorEnv)
	defer func() { _ = os.Setenv(GithubMirrorEnv, old) }()

	got, err := SetMirrorEnv(`"https://ghfast.top/"`)
	if err != nil {
		t.Fatalf("SetMirrorEnv() = %v", err)
	}
	if got != "https://ghfast.top" {
		t.Errorf("normalized = %q, want https://ghfast.top", got)
	}
	if env := os.Getenv(GithubMirrorEnv); env != "https://ghfast.top" {
		t.Errorf("env = %q, want https://ghfast.top", env)
	}

	// Empty input leaves the environment untouched.
	if _, err := SetMirrorEnv("   "); err != nil {
		t.Fatalf("SetMirrorEnv(blank) = %v", err)
	}
	if env := os.Getenv(GithubMirrorEnv); env != "https://ghfast.top" {
		t.Errorf("env changed by empty input: %q", env)
	}
}

func TestResolveDownloadURLTolerantMirror(t *testing.T) {
	old := os.Getenv(GithubMirrorEnv)
	defer func() { _ = os.Setenv(GithubMirrorEnv, old) }()

	// Mirrors exported by hand may carry quotes or trailing slashes.
	_ = os.Setenv(GithubMirrorEnv, `"https://gh-proxy.org/"`)
	got := ResolveDownloadURL("https://github.com/MetaCubeX/mihomo/releases/latest/download/version.txt")
	want := "https://gh-proxy.org/https://github.com/MetaCubeX/mihomo/releases/latest/download/version.txt"
	if got != want {
		t.Errorf("ResolveDownloadURL() = %s, want %s", got, want)
	}
}
