package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

// CreateParentDir ensures the parent directory of the given path exists.
func CreateParentDir(path string) error {
	parent := filepath.Dir(path)
	if parent == "." || parent == "/" {
		return nil
	}
	if err := os.MkdirAll(parent, 0755); err != nil {
		return fmt.Errorf("create parent dir of %s: %w", path, err)
	}
	return nil
}

// DeleteFile removes the file at path if it exists, logging with the given prefix.
func DeleteFile(path, prefix string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	fmt.Printf("%s Removed %s\n", prefix, path)
	return nil
}
