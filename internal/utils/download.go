package utils

import (
	"context"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	MaxRetries      = 3
	DetailPrefix    = "   "
	GithubMirrorEnv = "MIHORO_GITHUB_MIRROR"
)

// RetryStrategy returns exponential backoff delays with jitter.
func RetryStrategy(maxRetries int) []time.Duration {
	delays := make([]time.Duration, maxRetries)
	for i := range maxRetries {
		backoff := time.Duration(math.Pow(2, float64(i+1))) * 500 * time.Millisecond
		backoff = min(backoff, 5*time.Second)
		jitter := time.Duration(rand.Int63n(int64(backoff / 4)))
		delays[i] = backoff + jitter
	}
	return delays
}

// DownloadFile downloads a file with retry and a progress bar.
// label is shown on the left (e.g. "  subscribe   ").
func DownloadFile(ctx context.Context, client *http.Client, url, destPath, userAgent, label string) error {
	bar := NewProgressBar(label, 0)

	var lastErr error
	for attempt := 0; attempt <= MaxRetries; attempt++ {
		if attempt > 0 {
			bar.SetStatus(fmt.Sprintf("retry %d/%d", attempt, MaxRetries))
			delay := RetryStrategy(MaxRetries)[attempt-1]
			select {
			case <-ctx.Done():
				bar.Canceled()
				return ctx.Err()
			case <-time.After(delay):
			}
		}
		err := downloadOnce(ctx, client, url, destPath, userAgent, bar)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			bar.Canceled()
			return ctx.Err()
		}
		lastErr = err
	}
	bar.Failed()
	return fmt.Errorf("download %s failed after %d attempts: %v", strings.TrimSpace(label), MaxRetries+1, lastErr)
}

func downloadOnce(ctx context.Context, client *http.Client, url, destPath, userAgent string, bar *ProgressBar) error {
	resolvedURL := ResolveDownloadURL(url)

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resolvedURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("GET %s: %w", resolvedURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GET %s: HTTP %d", resolvedURL, resp.StatusCode)
	}

	bar.SetTotal(resp.ContentLength)

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create file %s: %w", destPath, err)
	}
	defer func() { _ = f.Close() }()

	_, err = io.Copy(f, io.TeeReader(resp.Body, bar))
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("download stream: %w", err)
	}
	bar.Done()

	return nil
}

// --- Mirror support ---

func githubMirrorBase() string {
	mirror := strings.TrimRight(os.Getenv(GithubMirrorEnv), "/")
	if mirror == "" {
		return ""
	}
	return mirror
}

func isGitHubDownloadHost(host string) bool {
	return host == "github.com" || strings.HasSuffix(host, ".githubusercontent.com")
}

func ResolveDownloadURL(url string) string {
	mirror := githubMirrorBase()
	if mirror == "" {
		return url
	}
	host := extractHost(url)
	if host == "" || !isGitHubDownloadHost(host) {
		return url
	}
	if strings.HasPrefix(url, mirror+"/") {
		return url
	}
	return mirror + "/" + url
}

func extractHost(rawURL string) string {
	s := rawURL
	if after, ok := strings.CutPrefix(s, "https://"); ok {
		s = after
	} else if after, ok := strings.CutPrefix(s, "http://"); ok {
		s = after
	}
	s, _, _ = strings.Cut(s, "/")
	return s
}
