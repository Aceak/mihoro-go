package utils

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
)

// ExtractGzip decompresses a gzip archive from srcPath and writes the content to destPath.
func ExtractGzip(srcPath, destPath, prefix string) error {
	if err := CreateParentDir(destPath); err != nil {
		return err
	}

	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open gzip %s: %w", srcPath, err)
	}
	defer func() { _ = src.Close() }()

	gz, err := gzip.NewReader(src)
	if err != nil {
		return fmt.Errorf("create gzip reader: %w", err)
	}
	defer func() { _ = gz.Close() }()

	dst, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create dest %s: %w", destPath, err)
	}
	defer func() { _ = dst.Close() }()

	if _, err := io.Copy(dst, gz); err != nil {
		return fmt.Errorf("decompress gzip: %w", err)
	}

	fmt.Printf("%sExtracted to %s\n", prefix, destPath)
	return nil
}
