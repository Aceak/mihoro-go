// Package upstream queries upstream release metadata (currently GitHub).
package upstream

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// APIBase is the GitHub API base URL. It is a variable so tests can point it
// at a local server.
var APIBase = "https://api.github.com"

const maxReleaseBody = 2 << 20 // 2 MiB

// Asset is a single file attached to a release.
type Asset struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	Digest string `json:"digest"` // e.g. "sha256:abcd..."; may be empty
}

// Release is the subset of GitHub release metadata mihoro needs.
type Release struct {
	Tag    string  `json:"tag_name"`
	Assets []Asset `json:"assets"`
}

// AssetByName returns the named asset, or nil when it is not present.
func (r *Release) AssetByName(name string) *Asset {
	for i := range r.Assets {
		if r.Assets[i].Name == name {
			return &r.Assets[i]
		}
	}
	return nil
}

// LatestRelease fetches the latest release of owner/repo from the GitHub API.
//
// The request goes directly to GitHub (it is never rewritten to a mirror);
// the supplied client may carry a proxy. GITHUB_TOKEN is used when set to
// avoid the low anonymous rate limit.
func LatestRelease(ctx context.Context, client *http.Client, owner, repo string) (*Release, error) {
	// The API response is small; cap the wait even if the shared client has
	// no overall timeout (downloads use their own timeout).
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}

	endpoint := fmt.Sprintf("%s/repos/%s/%s/releases/latest", strings.TrimRight(APIBase, "/"), owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "mihoro-go")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch release from %s: %w", endpoint, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch release from %s: HTTP %d", endpoint, resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxReleaseBody))
	if err != nil {
		return nil, fmt.Errorf("read release response: %w", err)
	}

	var rel Release
	if err := json.Unmarshal(data, &rel); err != nil {
		return nil, fmt.Errorf("parse release response: %w", err)
	}
	if rel.Tag == "" {
		return nil, fmt.Errorf("release response from %s has no tag", endpoint)
	}
	return &rel, nil
}
