package utils

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mihoro-go/internal/version"
)

const (
	MaxRetries      = 3
	DetailPrefix    = "   "
	GithubMirrorEnv = "MIHORO_GITHUB_MIRROR"
)

// DownloadOptions controls download behavior.
type DownloadOptions struct {
	URL       string            // download URL
	DestPath  string            // output file path (atomic: temp + rename)
	UserAgent string            // User-Agent header, default is "mihoro-go/<version>"
	Headers   map[string]string // extra request headers
	ProxyURL  string            // HTTP proxy URL, empty = no proxy
	Label     string            // progress bar label, empty = no progress bar
	Retries   int               // extra retries, 0 = no retry
	Timeout   time.Duration     // HTTP client timeout, 0 = default 30s
	// Validate is an optional content check run on the temp file before it is
	// renamed into place. A validation failure is not retried against the same
	// URL, so the mirror fallback can take over immediately.
	Validate func(path string) error
}

// ErrValidation marks a content validation failure.
var ErrValidation = errors.New("download validation failed")

func defaultUA() string {
	return "mihoro-go/" + version.Version
}

// Download downloads a file to DestPath (atomic write).
//
// When a GitHub mirror is configured, the mirrored URL is tried first. If it
// fails, the original URL is tried as a fallback so a dead mirror does not
// block updates.
func Download(ctx context.Context, client *http.Client, opts DownloadOptions) (size int64, err error) {
	if opts.UserAgent == "" {
		opts.UserAgent = defaultUA()
	}

	httpClient := getClient(client, opts.ProxyURL, opts.Timeout)

	var bar *ProgressBar
	if opts.Label != "" {
		bar = NewProgressBar(opts.Label, 0)
	}

	urls := []string{ResolveDownloadURL(opts.URL)}
	if urls[0] != opts.URL {
		urls = append(urls, opts.URL)
	}

	var lastErr error
	for i, u := range urls {
		if i > 0 {
			if bar != nil {
				bar.Reset()
				bar.SetStatus("mirror failed, trying direct")
			} else {
				fmt.Fprintln(os.Stderr, "mirror download failed, trying direct URL")
			}
		}
		n, err := downloadWithRetries(ctx, httpClient, opts, u, bar)
		if err == nil {
			return n, nil
		}
		if ctx.Err() != nil {
			if bar != nil {
				bar.Canceled()
			}
			return 0, ctx.Err()
		}
		lastErr = err
	}
	if bar != nil {
		bar.Failed()
	}
	if len(urls) > 1 {
		return 0, fmt.Errorf("download failed (mirror and direct): %v", lastErr)
	}
	return 0, fmt.Errorf("download failed after %d attempts: %v", opts.Retries+1, lastErr)
}

// downloadWithRetries downloads from a single URL, retrying with backoff.
func downloadWithRetries(ctx context.Context, client *http.Client, opts DownloadOptions, url string,
	bar *ProgressBar) (int64, error) {
	var lastErr error
	for attempt := 0; attempt <= opts.Retries; attempt++ {
		if attempt > 0 {
			delay := RetryStrategy(opts.Retries)[attempt-1]
			if bar != nil {
				bar.SetStatus(fmt.Sprintf("retry %d/%d", attempt, opts.Retries))
			}
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-time.After(delay):
			}
			if bar != nil {
				bar.Reset()
				bar.SetStatus(fmt.Sprintf("retry %d/%d", attempt, opts.Retries))
			}
		}
		n, err := downloadOnce(ctx, client, opts, url, bar)
		if err == nil {
			return n, nil
		}
		if errors.Is(err, ErrValidation) {
			// The content is wrong regardless of how often it is retried.
			return 0, err
		}
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		lastErr = err
	}
	return 0, lastErr
}

func downloadOnce(ctx context.Context, client *http.Client, opts DownloadOptions, resolvedURL string,
	bar *ProgressBar) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(opts.DestPath), 0755); err != nil {
		return 0, fmt.Errorf("create parent dir: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resolvedURL, nil)
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", opts.UserAgent)
	for k, v := range opts.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		return 0, fmt.Errorf("GET %s: %w", resolvedURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("GET %s: HTTP %d", resolvedURL, resp.StatusCode)
	}

	if bar != nil && resp.ContentLength > 0 {
		bar.SetTotal(resp.ContentLength)
	}

	// Atomic write: temp file then rename
	tmpPath := opts.DestPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return 0, fmt.Errorf("create temp: %w", err)
	}

	var writer io.Writer = f
	if bar != nil {
		writer = io.MultiWriter(f, bar)
	}

	written, err := io.Copy(writer, resp.Body)
	if err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		return 0, fmt.Errorf("download stream: %w", err)
	}
	_ = f.Close()

	if written == 0 {
		_ = os.Remove(tmpPath)
		return 0, fmt.Errorf("empty response")
	}

	if opts.Validate != nil {
		if err := opts.Validate(tmpPath); err != nil {
			_ = os.Remove(tmpPath)
			return 0, fmt.Errorf("%w: %v", ErrValidation, err)
		}
	}

	if err := os.Rename(tmpPath, opts.DestPath); err != nil {
		_ = os.Remove(tmpPath)
		return 0, fmt.Errorf("rename: %w", err)
	}

	if bar != nil {
		bar.Done()
	}
	return written, nil
}

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

// --- Mirror support ---

func githubMirrorBase() string {
	// Tolerate manually exported values: strip whitespace, quotes and
	// trailing slashes so URL concatenation always yields a single "/".
	mirror := strings.TrimSpace(os.Getenv(GithubMirrorEnv))
	mirror = strings.Trim(mirror, `"'`)
	return strings.TrimRight(mirror, "/")
}

func isGitHubDownloadHost(host string) bool {
	return host == "github.com" || strings.HasSuffix(host, ".githubusercontent.com")
}

// ResolveDownloadURL rewrites GitHub URLs to a mirror if configured.
func ResolveDownloadURL(rawURL string) string {
	mirror := githubMirrorBase()
	if mirror == "" {
		return rawURL
	}
	host := extractHost(rawURL)
	if host == "" || !isGitHubDownloadHost(host) {
		return rawURL
	}
	if strings.HasPrefix(rawURL, mirror+"/") {
		return rawURL
	}
	return mirror + "/" + rawURL
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

// getClient returns a copy of client with the requested timeout and proxy.
// It never mutates the shared client, so a caller-provided Timeout is honored
// even when the client is shared (previously it was ignored).
func getClient(client *http.Client, proxyURL string, timeout time.Duration) *http.Client {
	var c http.Client
	if client != nil {
		c = *client
	}
	if timeout > 0 {
		c.Timeout = timeout
	} else if client == nil {
		c.Timeout = 10 * time.Second
	}
	if proxyURL != "" {
		if pu, err := url.Parse(proxyURL); err == nil {
			var tr *http.Transport
			if existing, ok := c.Transport.(*http.Transport); ok {
				tr = existing.Clone()
			} else if base, ok := http.DefaultTransport.(*http.Transport); ok {
				tr = base.Clone()
			}
			if tr != nil {
				tr.Proxy = http.ProxyURL(pu)
				c.Transport = tr
			}
		}
	}
	return &c
}
