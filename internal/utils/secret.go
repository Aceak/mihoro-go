package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// SecretBytes is the number of random bytes in a generated API secret.
// 16 bytes produce 32 hex characters (128 bits of entropy).
const SecretBytes = 16

// GenerateSecret returns a cryptographically secure random hex string
// (2*SecretBytes characters) suitable as a mihomo API secret.
func GenerateSecret() (string, error) {
	buf := make([]byte, SecretBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
