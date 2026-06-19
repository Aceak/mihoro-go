package upgrade

import (
	"strings"
	"testing"

	"mihoro-go/internal/version"
)

func TestVersionDefault(t *testing.T) {
	if version.Version == "" {
		t.Error("Version should not be empty")
	}
}

func TestBuildTarget(t *testing.T) {
	target := buildTarget()
	if !strings.HasPrefix(target, "linux-") {
		t.Errorf("target should start with linux-, got: %s", target)
	}
}

func TestFindAsset(t *testing.T) {
	assets := []githubAsset{
		{Name: "mihoro-linux-amd64", BrowserDownloadURL: "https://example.com/asset1"},
		{Name: "mihoro-linux-arm64", BrowserDownloadURL: "https://example.com/asset2"},
		{Name: "other-file.txt", BrowserDownloadURL: "https://example.com/ignored"},
	}

	url := findAsset(assets, "linux-amd64")
	if url != "https://example.com/asset1" {
		t.Errorf("got %s, want asset1", url)
	}

	url = findAsset(assets, "linux-arm64")
	if url != "https://example.com/asset2" {
		t.Errorf("got %s, want asset2", url)
	}

	url = findAsset(assets, "linux-armv7")
	if url != "" {
		t.Errorf("expected empty for missing target, got %s", url)
	}
}
