package utils

import (
	"strings"
	"testing"
)

func TestNormalizeProxy(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"empty", "", "", false},
		{"blank", "   ", "", false},
		{"no scheme", "127.0.0.1:7890", "http://127.0.0.1:7890", false},
		{"http", "http://127.0.0.1:7890", "http://127.0.0.1:7890", false},
		{"https", "https://proxy.example.com:443", "https://proxy.example.com:443", false},
		{"socks5", "socks5://127.0.0.1:1080", "socks5://127.0.0.1:1080", false},
		{"socks5h", "socks5h://127.0.0.1:1080", "socks5h://127.0.0.1:1080", false},
		{"quoted", `"http://127.0.0.1:7890"`, "http://127.0.0.1:7890", false},
		{"credentials", "http://user:pass@127.0.0.1:7890", "http://user:pass@127.0.0.1:7890", false},
		{"unsupported scheme", "ftp://127.0.0.1:21", "", true},
		{"missing host", "http://", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeProxy(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NormalizeProxy(%q) = %q, want error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeProxy(%q) = %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("NormalizeProxy(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRedactProxy(t *testing.T) {
	got := RedactProxy("http://user:supersecret@127.0.0.1:7890")
	if strings.Contains(got, "supersecret") {
		t.Errorf("RedactProxy leaked the password: %s", got)
	}
	if !strings.Contains(got, "xxxxx") {
		t.Errorf("RedactProxy() = %q, want redacted password", got)
	}
}
