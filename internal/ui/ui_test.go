package ui

import (
	"os"
	"testing"
)

func TestResolveExternalUIPath(t *testing.T) {
	result := ResolveExternalUIPath("/tmp/mihomo", "ui")
	if result != "/tmp/mihomo/ui" {
		t.Errorf("relative: got %s, want /tmp/mihomo/ui", result)
	}

	result = ResolveExternalUIPath("/tmp/mihomo", "/var/www/ui")
	if result != "/var/www/ui" {
		t.Errorf("absolute: got %s, want /var/www/ui", result)
	}
}

func TestFindArchiveRoot(t *testing.T) {
	dir := t.TempDir()
	sub := dir + "/gh-pages"
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}

	root, err := findArchiveRoot(dir)
	if err != nil {
		t.Fatalf("findArchiveRoot() = %v", err)
	}
	if root != sub {
		t.Errorf("got %s, want %s", root, sub)
	}
}

func TestFindArchiveRootMultipleEntries(t *testing.T) {
	dir := t.TempDir()
	_ = os.Mkdir(dir+"/a", 0755)
	_ = os.Mkdir(dir+"/b", 0755)

	root, err := findArchiveRoot(dir)
	if err != nil {
		t.Fatalf("findArchiveRoot() = %v", err)
	}
	// Multiple entries should return the extract dir itself
	if root != dir {
		t.Errorf("got %s, want %s", root, dir)
	}
}

func TestReplaceDir(t *testing.T) {
	dir := t.TempDir()

	src := dir + "/source"
	dst := dir + "/target"

	_ = os.Mkdir(src, 0755)
	_ = os.WriteFile(src+"/index.html", []byte("<html></html>"), 0644)

	if err := replaceDir(src, dst); err != nil {
		t.Fatalf("replaceDir() = %v", err)
	}

	// Verify destination exists with content
	data, err := os.ReadFile(dst + "/index.html")
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	if string(data) != "<html></html>" {
		t.Errorf("content = %s, want <html></html>", string(data))
	}

	// Source should be gone
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("source should have been moved")
	}
}

func TestReplaceDirOverwritesExisting(t *testing.T) {
	dir := t.TempDir()

	src := dir + "/source"
	dst := dir + "/target"

	_ = os.Mkdir(src, 0755)
	_ = os.WriteFile(src+"/new.html", []byte("new"), 0644)

	_ = os.Mkdir(dst, 0755)
	_ = os.WriteFile(dst+"/old.html", []byte("old"), 0644)

	if err := replaceDir(src, dst); err != nil {
		t.Fatalf("replaceDir() = %v", err)
	}

	// New content should be present
	if _, err := os.Stat(dst + "/new.html"); os.IsNotExist(err) {
		t.Error("new.html should exist")
	}
	// Old content should be gone
	if _, err := os.Stat(dst + "/old.html"); !os.IsNotExist(err) {
		t.Error("old.html should not exist")
	}
}
