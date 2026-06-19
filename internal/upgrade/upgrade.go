package upgrade

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"

	"mihoro-go/internal/version"

	"github.com/minio/selfupdate"
)

const (
	repoOwner = "Aceak"
	repoName  = "mihoro-go"
)

// --- GitHub API types ---

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// --- Public API ---

// CheckForUpdate checks GitHub for a newer mihoro release.
// Returns the latest version string if an update is available, or empty string.
func CheckForUpdate(ctx context.Context, client *http.Client) (string, error) {
	latest, err := fetchLatestRelease(ctx, client)
	if err != nil {
		return "", err
	}
	current := strings.TrimPrefix(version.Version, "v")
	tag := strings.TrimPrefix(latest.TagName, "v")
	if tag != current {
		return "v" + tag, nil
	}
	return "", nil
}

// RunUpgrade downloads and installs the latest mihoro release.
func RunUpgrade(ctx context.Context, client *http.Client) error {
	latest, err := fetchLatestRelease(ctx, client)
	if err != nil {
		return err
	}

	target := buildTarget()
	assetURL := findAsset(latest.Assets, target)
	if assetURL == "" {
		return fmt.Errorf("no release asset found for %s in %s", target, latest.TagName)
	}

	fmt.Printf("mihoro: Downloading %s for %s...\n", latest.TagName, target)

	req, err := http.NewRequestWithContext(ctx, "GET", assetURL, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download release: HTTP %d", resp.StatusCode)
	}

	binData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read release: %w", err)
	}

	if err := selfupdate.Apply(bytes.NewReader(binData), selfupdate.Options{}); err != nil {
		if rerr := selfupdate.RollbackError(err); rerr != nil {
			return fmt.Errorf("update failed + rollback failed: %v (original: %w)", rerr, err)
		}
		return fmt.Errorf("update failed (rolled back): %w", err)
	}

	fmt.Printf("mihoro: Updated to %s\n", latest.TagName)
	fmt.Println("mihoro: Please restart for the new version to take effect")
	return nil
}

// --- internal helpers ---

func buildTarget() string {
	arch := runtime.GOARCH
	switch arch {
	case "arm":
		arch = "armv7"
	}
	return "linux-" + arch
}

func findAsset(assets []githubAsset, target string) string {
	needle := "mihoro-" + target
	for _, a := range assets {
		if a.Name == needle {
			return a.BrowserDownloadURL
		}
	}
	// Fallback: partial match
	for _, a := range assets {
		if strings.Contains(a.Name, target) {
			return a.BrowserDownloadURL
		}
	}
	return ""
}

func fetchLatestRelease(ctx context.Context, client *http.Client) (*githubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch releases: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fetch releases: HTTP %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("parse release JSON: %w", err)
	}
	return &release, nil
}
