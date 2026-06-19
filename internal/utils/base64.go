package utils

import (
	"encoding/base64"
	"fmt"
	"os"
)

// TryDecodeBase64InPlace attempts to decode the file at filepath as base64 in place.
// If the file is not valid base64, it is left unchanged and no error is returned.
func TryDecodeBase64InPlace(filepath string) error {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("read file %s: %w", filepath, err)
	}

	decoded, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		// Not base64 — leave unchanged
		return nil
	}

	if err := os.WriteFile(filepath, decoded, 0644); err != nil {
		return fmt.Errorf("write decoded file %s: %w", filepath, err)
	}
	return nil
}
