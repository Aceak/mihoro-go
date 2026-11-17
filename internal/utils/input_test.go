package utils

import "testing"

func TestStripQuotes(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"https://x", "https://x"},
		{`"https://x"`, "https://x"},
		{`'https://x'`, "https://x"},
		{"  https://x  ", "https://x"},
		{`" spaced "`, "spaced"},
		{" ", ""},
		{"", ""},
	}

	for _, tt := range tests {
		if got := StripQuotes(tt.in); got != tt.want {
			t.Errorf("StripQuotes(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNormalizeURLInput(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"plain", "https://example.com/sub", "https://example.com/sub", false},
		{"double quoted", `"https://example.com/sub"`, "https://example.com/sub", false},
		{"single quoted", `'https://example.com/sub'`, "https://example.com/sub", false},
		{"http scheme", "http://example.com", "http://example.com", false},
		{"empty", "", "", false},
		{"quoted empty", `""`, "", false},
		{"no scheme", "example.com/sub", "", true},
		{"unsupported scheme", "ftp://example.com", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeURLInput(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("NormalizeURLInput(%q) = %q, want error", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeURLInput(%q) = %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("NormalizeURLInput(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
