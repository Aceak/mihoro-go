package utils

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// roundTripFunc lets a test answer requests for any host without network I/O.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestGetClientHonorsTimeout(t *testing.T) {
	shared := &http.Client{}
	got := getClient(shared, "", 30*time.Second)
	if got.Timeout != 30*time.Second {
		t.Errorf("timeout = %v, want 30s", got.Timeout)
	}
	if shared.Timeout != 0 {
		t.Errorf("shared client was mutated: timeout = %v", shared.Timeout)
	}
}

func TestGetClientAppliesProxy(t *testing.T) {
	got := getClient(&http.Client{}, "http://127.0.0.1:7890", 5*time.Second)
	tr, ok := got.Transport.(*http.Transport)
	if !ok || tr.Proxy == nil {
		t.Fatalf("transport = %#v, want a proxying transport", got.Transport)
	}
	req, _ := http.NewRequest(http.MethodGet, "https://github.com/x", nil)
	proxyURL, err := tr.Proxy(req)
	if err != nil {
		t.Fatalf("proxy lookup: %v", err)
	}
	if proxyURL == nil || proxyURL.String() != "http://127.0.0.1:7890" {
		t.Errorf("proxy = %v, want http://127.0.0.1:7890", proxyURL)
	}
}

func TestDownloadFallsBackToDirect(t *testing.T) {
	const mirrorHost = "mirror.test"

	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host == mirrorHost {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Body:       io.NopCloser(strings.NewReader("mirror down")),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("direct-content")),
			Header:     make(http.Header),
		}, nil
	})

	t.Setenv(GithubMirrorEnv, "https://"+mirrorHost)

	dest := filepath.Join(t.TempDir(), "file.bin")
	size, err := Download(context.Background(), &http.Client{Transport: transport}, DownloadOptions{
		URL:      "https://github.com/example/file.bin",
		DestPath: dest,
		Retries:  0,
	})
	if err != nil {
		t.Fatalf("Download() = %v", err)
	}
	if size != int64(len("direct-content")) {
		t.Errorf("size = %d, want %d", size, len("direct-content"))
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "direct-content" {
		t.Errorf("content = %q, want direct-content", data)
	}
}

func TestDownloadFailureLeavesNoFile(t *testing.T) {
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader("error")),
			Header:     make(http.Header),
		}, nil
	})

	dest := filepath.Join(t.TempDir(), "file.bin")
	if _, err := Download(context.Background(), &http.Client{Transport: transport}, DownloadOptions{
		URL:      "https://example.com/file.bin",
		DestPath: dest,
		Retries:  0,
	}); err == nil {
		t.Fatal("expected an error")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("destination should not exist after failure, stat err = %v", err)
	}
}

func TestWriteFileAtomic(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "state", "file.toml")
	if err := WriteFileAtomic(dest, []byte("value = 1\n"), 0644); err != nil {
		t.Fatalf("WriteFileAtomic() = %v", err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "value = 1\n" {
		t.Errorf("content = %q", data)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0644 {
		t.Errorf("perm = %o, want 644", info.Mode().Perm())
	}
}

func TestDownloadValidationTriggersDirectFallback(t *testing.T) {
	const mirrorHost = "mirror.test"
	var mirrorCalls atomic.Int32

	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host == mirrorHost {
			mirrorCalls.Add(1)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("<html>mirror error page</html>")),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("\x1f\x8bgzip-data")),
			Header:     make(http.Header),
		}, nil
	})

	t.Setenv(GithubMirrorEnv, "https://"+mirrorHost)

	dest := filepath.Join(t.TempDir(), "core.gz")
	_, err := Download(context.Background(), &http.Client{Transport: transport}, DownloadOptions{
		URL:      "https://github.com/example/core.gz",
		DestPath: dest,
		Retries:  3,
		Validate: func(path string) error {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if len(data) < 2 || data[0] != 0x1f || data[1] != 0x8b {
				return errors.New("not gzip")
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Download() = %v", err)
	}
	if mirrorCalls.Load() != 1 {
		t.Errorf("mirror was called %d times, want 1 (validation failure must not retry)", mirrorCalls.Load())
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "\x1f\x8bgzip-data" {
		t.Errorf("content = %q, want the direct download", data)
	}
}
