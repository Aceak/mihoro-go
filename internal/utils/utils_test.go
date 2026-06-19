package utils

import (
	"compress/gzip"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestRetryStrategy(t *testing.T) {
	delays := RetryStrategy(3)
	if len(delays) != 3 {
		t.Errorf("len = %d, want 3", len(delays))
	}
	for i, d := range delays {
		if d <= 0 {
			t.Errorf("delay[%d] = %v, want positive", i, d)
		}
		if d > 6*1e9 { // cap + jitter should not exceed ~6.25s
			t.Errorf("delay[%d] = %v, too large", i, d)
		}
	}
}

func TestCreateParentDir(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b", "file.txt")

	if err := CreateParentDir(nested); err != nil {
		t.Fatalf("CreateParentDir() = %v", err)
	}

	parent := filepath.Dir(nested)
	if _, err := os.Stat(parent); os.IsNotExist(err) {
		t.Error("parent directory was not created")
	}
}

func TestDeleteFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := DeleteFile(path, "prefix"); err != nil {
		t.Fatalf("DeleteFile() = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should not exist after delete")
	}

	// Should not error on non-existent file
	if err := DeleteFile(path, "prefix"); err != nil {
		t.Fatalf("DeleteFile() on missing = %v", err)
	}
}

func TestExtractGzip(t *testing.T) {
	dir := t.TempDir()
	gzipPath := filepath.Join(dir, "test.gz")
	outPath := filepath.Join(dir, "output.txt")

	// Create a gzip file
	f, err := os.Create(gzipPath)
	if err != nil {
		t.Fatal(err)
	}
	gw := gzip.NewWriter(f)
	if _, err := gw.Write([]byte("hello world")); err != nil {
		t.Fatal(err)
	}
	_ = gw.Close()
	_ = f.Close()

	if err := ExtractGzip(gzipPath, outPath, "prefix"); err != nil {
		t.Fatalf("ExtractGzip() = %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world" {
		t.Errorf("content = %q, want %q", string(data), "hello world")
	}
}

func TestTryDecodeBase64InPlace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	// Valid base64
	encoded := base64.StdEncoding.EncodeToString([]byte("hello base64"))
	if err := os.WriteFile(path, []byte(encoded), 0644); err != nil {
		t.Fatal(err)
	}
	if err := TryDecodeBase64InPlace(path); err != nil {
		t.Fatalf("TryDecodeBase64InPlace() = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello base64" {
		t.Errorf("decoded = %q, want %q", string(data), "hello base64")
	}

	// Invalid base64 — should be left unchanged
	invalid := "!!! not base64 !!!"
	if err := os.WriteFile(path, []byte(invalid), 0644); err != nil {
		t.Fatal(err)
	}
	if err := TryDecodeBase64InPlace(path); err != nil {
		t.Fatalf("TryDecodeBase64InPlace() on invalid = %v", err)
	}
	data, _ = os.ReadFile(path)
	if string(data) != invalid {
		t.Errorf("invalid base64 should be unchanged, got %q", string(data))
	}
}

func TestResolveDownloadURL(t *testing.T) {
	// Save and restore env
	oldMirror := os.Getenv(GithubMirrorEnv)
	defer func() { _ = os.Setenv(GithubMirrorEnv, oldMirror) }()

	_ = os.Setenv(GithubMirrorEnv, "https://gh-proxy.org")

	if got := ResolveDownloadURL(
		"https://github.com/MetaCubeX/mihomo/releases/latest/download/version.txt",
	); got != "https://gh-proxy.org/https://github.com/MetaCubeX/mihomo/releases/latest/download/version.txt" {
		t.Errorf("github download not mirrored: %s", got)
	}

	if got := ResolveDownloadURL("https://example.com/file.tar.gz"); got != "https://example.com/file.tar.gz" {
		t.Errorf("non-github URL should not be mirrored: %s", got)
	}

	if got := ResolveDownloadURL("https://api.github.com/repos/aceak/mihoro-go/releases/latest"); got != "https://api.github.com/repos/aceak/mihoro-go/releases/latest" {
		t.Errorf("api.github.com should not be mirrored: %s", got)
	}

	_ = os.Unsetenv(GithubMirrorEnv)
	if got := ResolveDownloadURL("https://github.com/x/y"); got != "https://github.com/x/y" {
		t.Errorf("URL unchanged without mirror: %s", got)
	}
}

func TestRetryStrategyBoundaries(t *testing.T) {
	delays := RetryStrategy(0)
	if len(delays) != 0 {
		t.Errorf("0 retries should produce 0 delays, got %d", len(delays))
	}
	delays = RetryStrategy(10)
	if len(delays) != 10 {
		t.Errorf("expected 10 delays, got %d", len(delays))
	}
	for i, d := range delays {
		if d > 7*1e9 {
			t.Errorf("delay[%d] = %v exceeds max", i, d)
		}
	}
}

func TestExtractGzipInvalidInput(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(dir+"/bad.gz", []byte("not a gzip file"), 0644)
	err := ExtractGzip(dir+"/bad.gz", dir+"/out", "p")
	if err == nil {
		t.Error("expected error for invalid gzip input")
	}
}

func TestExtractGzipMissingFile(t *testing.T) {
	dir := t.TempDir()
	err := ExtractGzip(dir+"/nonexistent.gz", dir+"/out", "p")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestMirrorAlreadyPrefixed(t *testing.T) {
	oldMirror := os.Getenv(GithubMirrorEnv)
	defer func() { _ = os.Setenv(GithubMirrorEnv, oldMirror) }()
	_ = os.Setenv(GithubMirrorEnv, "https://gh-proxy.org")
	result := ResolveDownloadURL("https://gh-proxy.org/https://github.com/x/y")
	if result == "https://gh-proxy.org/https://gh-proxy.org/https://github.com/x/y" {
		t.Error("should not double-prefix mirror")
	}
}

func TestCreateParentDirRoot(t *testing.T) {
	if err := CreateParentDir("/file-at-root"); err != nil {
		t.Errorf("root parent should be ok: %v", err)
	}
}

func TestDeleteFileMissing(t *testing.T) {
	dir := t.TempDir()
	if err := DeleteFile(dir+"/nonexistent.txt", "p"); err != nil {
		t.Errorf("delete missing file should not error: %v", err)
	}
}
