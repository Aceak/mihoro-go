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

// WriteFileAtomic writes data to path via a temp file in the same directory
// followed by a rename, so readers never observe a partially written file.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	if err := CreateParentDir(path); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file for %s: %w", path, err)
	}
	tmpName := f.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := f.Chmod(perm); err != nil {
		_ = f.Close()
		return fmt.Errorf("chmod %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close %s: %w", path, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename %s: %w", path, err)
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
