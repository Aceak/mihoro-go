package utils

import (
	"encoding/hex"
	"testing"
)

func TestGenerateSecretLengthAndCharset(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret() = %v", err)
	}
	if len(secret) != 2*SecretBytes {
		t.Errorf("len = %d, want %d", len(secret), 2*SecretBytes)
	}
	if _, err := hex.DecodeString(secret); err != nil {
		t.Errorf("secret is not valid hex: %v", err)
	}
}

func TestGenerateSecretUnique(t *testing.T) {
	seen := make(map[string]bool)
	for range 64 {
		secret, err := GenerateSecret()
		if err != nil {
			t.Fatalf("GenerateSecret() = %v", err)
		}
		if seen[secret] {
			t.Fatalf("duplicate secret generated: %s", secret)
		}
		seen[secret] = true
	}
}
