package ui

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"mihoro-go/internal/config"
	"mihoro-go/internal/utils"
)

// InstallUI downloads the dashboard archive and extracts it to targetDir atomically.
func InstallUI(ctx context.Context, client *http.Client, ui config.Ui, targetDir, userAgent, prefix string) error {
	// Download to temp file
	tmpFile, err := os.CreateTemp("", "mihoro-ui-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := utils.DownloadFile(ctx, client, ui.DownloadURL(), tmpPath, userAgent, "  web ui     "); err != nil {
		return fmt.Errorf("download ui %s: %w", ui.AsConfigValue(), err)
	}

	// Extract to temp directory
	extractDir, err := os.MkdirTemp(filepath.Dir(targetDir), ".mihoro-ui-extract-*")
	if err != nil {
		return fmt.Errorf("create extract dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(extractDir) }()

	if strings.HasSuffix(ui.DownloadURL(), ".zip") {
		if err := extractZip(tmpPath, extractDir); err != nil {
			return fmt.Errorf("extract ui archive: %w", err)
		}
	} else {
		if err := extractTarGz(tmpPath, extractDir); err != nil {
			return fmt.Errorf("extract ui archive: %w", err)
		}
	}

	// Find the single root directory in the archive
	rootDir, err := findArchiveRoot(extractDir)
	if err != nil {
		return fmt.Errorf("find archive root: %w", err)
	}

	// Atomic replace
	if err := replaceDir(rootDir, targetDir); err != nil {
		return fmt.Errorf("replace ui dir: %w", err)
	}

	fmt.Printf("%sInstalled UI %q to %s\n", prefix, ui.AsConfigValue(), targetDir)
	return nil
}

// ResolveExternalUIPath resolves the dashboard directory path.
// If externalUI is an absolute path, returns it directly;
// otherwise joins it with configRoot.
func ResolveExternalUIPath(configRoot, externalUI string) string {
	if filepath.IsAbs(externalUI) {
		return externalUI
	}
	return filepath.Join(configRoot, externalUI)
}

// --- internal helpers ---

func extractZip(archivePath, destDir string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer func() { _ = r.Close() }()

	for _, f := range r.File {
		target := filepath.Join(destDir, f.Name)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)+string(os.PathSeparator)) {
			continue
		}
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(target, 0755)
			continue
		}
		_ = os.MkdirAll(filepath.Dir(target), 0755)
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("open zip entry %s: %w", f.Name, err)
		}
		out, err := os.Create(target)
		if err != nil {
			_ = rc.Close()
			return fmt.Errorf("create %s: %w", target, err)
		}
		_, err = io.Copy(out, rc)
		_ = rc.Close()
		_ = out.Close()
		if err != nil {
			return fmt.Errorf("write %s: %w", target, err)
		}
	}
	return nil
}

func extractTarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}

		target := filepath.Join(destDir, hdr.Name)
		// Prevent path traversal
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)+string(os.PathSeparator)) {
			continue
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("mkdir %s: %w", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("mkdir parent %s: %w", filepath.Dir(target), err)
			}
			out, err := os.Create(target)
			if err != nil {
				return fmt.Errorf("create %s: %w", target, err)
			}
			if _, err := io.Copy(out, tr); err != nil {
				_ = out.Close()
				return fmt.Errorf("write %s: %w", target, err)
			}
			_ = out.Close()
			if err := os.Chmod(target, os.FileMode(hdr.Mode)); err != nil {
				return fmt.Errorf("chmod %s: %w", target, err)
			}
		}
	}
	return nil
}

func findArchiveRoot(extractDir string) (string, error) {
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return "", fmt.Errorf("read extract dir: %w", err)
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("empty archive")
	}
	// Single root directory (e.g. gh-pages/) — return it
	if len(entries) == 1 && entries[0].IsDir() {
		return filepath.Join(extractDir, entries[0].Name()), nil
	}
	// Multiple entries (e.g. compressed-dist) — return the extract dir itself
	return extractDir, nil
}

// replaceDir atomically replaces targetDir with the contents of sourceDir.
// Uses staging and backup directories to avoid broken states.
func replaceDir(sourceDir, targetDir string) error {
	if err := os.MkdirAll(filepath.Dir(targetDir), 0755); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}

	parent := filepath.Dir(targetDir)
	base := filepath.Base(targetDir)
	staged := filepath.Join(parent, "."+base+".tmp")
	backup := filepath.Join(parent, "."+base+".bak")

	// Clean up leftover staging/backup
	_ = os.RemoveAll(staged)
	_ = os.RemoveAll(backup)

	// Move source → staged
	if err := os.Rename(sourceDir, staged); err != nil {
		return fmt.Errorf("stage new ui: %w", err)
	}

	// Move target → backup (if target exists)
	targetExists := false
	if _, err := os.Stat(targetDir); err == nil {
		if err := os.Rename(targetDir, backup); err != nil {
			_ = os.Rename(staged, sourceDir) // try to restore
			return fmt.Errorf("backup existing ui: %w", err)
		}
		targetExists = true
	}

	// Move staged → target
	if err := os.Rename(staged, targetDir); err != nil {
		if targetExists {
			_ = os.Rename(backup, targetDir) // restore on failure
		}
		return fmt.Errorf("install new ui: %w", err)
	}

	// Clean up backup
	if targetExists {
		_ = os.RemoveAll(backup)
	}
	return nil
}
