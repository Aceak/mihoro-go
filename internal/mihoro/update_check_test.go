package mihoro

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"mihoro-go/internal/config"
	"mihoro-go/internal/upstream"
)

// withReleaseAPI points upstream.APIBase at a test server returning the given
// release. assets maps an asset name to its sha256 (without the prefix).
func withReleaseAPI(t *testing.T, tag string, assets map[string]string) *atomic.Int32 {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		type asset struct {
			Name   string `json:"name"`
			Size   int64  `json:"size"`
			Digest string `json:"digest"`
		}
		rel := struct {
			Tag    string  `json:"tag_name"`
			Assets []asset `json:"assets"`
		}{Tag: tag}
		for name, sum := range assets {
			rel.Assets = append(rel.Assets, asset{Name: name, Size: 1, Digest: "sha256:" + sum})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rel)
	}))
	old := upstream.APIBase
	upstream.APIBase = srv.URL
	t.Cleanup(func() {
		upstream.APIBase = old
		srv.Close()
	})
	return &calls
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestGeodataUpToDateBySHA256(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "country.mmdb")
	content := []byte("mmdb-binary-content")
	if err := os.WriteFile(dest, content, 0644); err != nil {
		t.Fatal(err)
	}
	withReleaseAPI(t, "latest", map[string]string{"country.mmdb": sha256Hex(content)})

	m := &Mihoro{ConfigDir: dir, ConfigRoot: dir}
	targets := []geodataTarget{{
		url:       "https://github.com/MetaCubeX/meta-rules-dat/releases/latest/download/country.mmdb",
		dest:      dest,
		assetName: "country.mmdb",
	}}

	ok, err := m.geodataUpToDate(context.Background(), nil, targets)
	if err != nil {
		t.Fatalf("geodataUpToDate() = %v", err)
	}
	if !ok {
		t.Error("expected up to date when the local sha256 matches")
	}

	// A modified local file must trigger a download.
	if err := os.WriteFile(dest, []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	ok, err = m.geodataUpToDate(context.Background(), nil, targets)
	if err != nil {
		t.Fatalf("geodataUpToDate() = %v", err)
	}
	if ok {
		t.Error("expected an update when the local sha256 differs")
	}
}

func TestGeodataUpToDateHandlesMissingInputs(t *testing.T) {
	dir := t.TempDir()
	withReleaseAPI(t, "latest", nil)
	m := &Mihoro{ConfigDir: dir, ConfigRoot: dir}

	// Missing local file.
	missing := []geodataTarget{{
		url:       "https://github.com/MetaCubeX/meta-rules-dat/releases/latest/download/country.mmdb",
		dest:      filepath.Join(dir, "country.mmdb"),
		assetName: "country.mmdb",
	}}
	if ok, err := m.geodataUpToDate(context.Background(), nil, missing); err != nil || ok {
		t.Errorf("missing file: ok=%v err=%v, want false, nil", ok, err)
	}

	// Asset without a digest.
	other := []geodataTarget{{
		url:       "https://github.com/MetaCubeX/meta-rules-dat/releases/latest/download/geoip.dat",
		dest:      filepath.Join(dir, "geoip.dat"),
		assetName: "geoip.dat",
	}}
	if ok, err := m.geodataUpToDate(context.Background(), nil, other); err != nil || ok {
		t.Errorf("missing asset: ok=%v err=%v, want false, nil", ok, err)
	}
}

func TestGeodataUpToDateSkipsNonDefaultSource(t *testing.T) {
	dir := t.TempDir()
	calls := withReleaseAPI(t, "latest", nil)
	m := &Mihoro{ConfigDir: dir, ConfigRoot: dir}
	targets := []geodataTarget{{
		url:       "https://example.com/country.mmdb",
		dest:      filepath.Join(dir, "country.mmdb"),
		assetName: "country.mmdb",
	}}
	ok, err := m.geodataUpToDate(context.Background(), nil, targets)
	if err != nil || ok {
		t.Errorf("custom source: ok=%v err=%v, want false, nil", ok, err)
	}
	if calls.Load() != 0 {
		t.Errorf("API was called %d times for a custom source", calls.Load())
	}
}

func TestUIUpToDate(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "ui")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	withReleaseAPI(t, "v1.274.6", nil)
	m := &Mihoro{ConfigDir: dir}

	st := newUpdateState()
	st.UI["metacubexd"] = "v1.274.6"
	if err := saveUpdateState(dir, st); err != nil {
		t.Fatal(err)
	}

	tag, upToDate, checked, err := m.uiUpToDate(context.Background(), nil, config.UiMetacubexd, target)
	if err != nil || !checked {
		t.Fatalf("uiUpToDate() = tag %q checked %v err %v", tag, checked, err)
	}
	if !upToDate || tag != "v1.274.6" {
		t.Errorf("tag = %q, upToDate = %v, want v1.274.6, true", tag, upToDate)
	}

	// A stale state tag must trigger a download.
	st.UI["metacubexd"] = "v1.0.0"
	if err := saveUpdateState(dir, st); err != nil {
		t.Fatal(err)
	}
	if _, upToDate, _, err := m.uiUpToDate(context.Background(), nil, config.UiMetacubexd, target); err != nil || upToDate {
		t.Errorf("stale tag: upToDate=%v err=%v, want false, nil", upToDate, err)
	}
}

func TestUIUpToDateUnsupportedSource(t *testing.T) {
	calls := withReleaseAPI(t, "v1.0.0", nil)
	m := &Mihoro{ConfigDir: t.TempDir()}
	for _, ui := range []config.Ui{config.UiYacdMeta, config.Ui("custom:https://example.com/ui.zip")} {
		_, _, checked, err := m.uiUpToDate(context.Background(), nil, ui, t.TempDir())
		if err != nil || checked {
			t.Errorf("ui %q: checked=%v err=%v, want false, nil", ui, checked, err)
		}
	}
	if calls.Load() != 0 {
		t.Errorf("API was called %d times for unsupported UIs", calls.Load())
	}
}

func TestValidateGeodataFile(t *testing.T) {
	dir := t.TempDir()
	html := filepath.Join(dir, "html")
	if err := os.WriteFile(html, []byte("  <!DOCTYPE html><html>error</html>"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := validateGeodataFile(html); err == nil {
		t.Error("expected HTML content to be rejected")
	}

	binary := filepath.Join(dir, "binary")
	if err := os.WriteFile(binary, []byte{0x00, 0x01, 0x02, 0xff}, 0644); err != nil {
		t.Fatal(err)
	}
	if err := validateGeodataFile(binary); err != nil {
		t.Errorf("binary content rejected: %v", err)
	}

	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := validateGeodataFile(empty); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("empty file error = %v, want an empty-file error", err)
	}
}

func TestValidateGzipAndZip(t *testing.T) {
	dir := t.TempDir()

	gz := filepath.Join(dir, "archive.gz")
	if err := os.WriteFile(gz, []byte{0x1f, 0x8b, 0x08, 0x00}, 0644); err != nil {
		t.Fatal(err)
	}
	if err := validateGzipFile(gz); err != nil {
		t.Errorf("valid gzip rejected: %v", err)
	}

	zipPath := filepath.Join(dir, "archive.zip")
	if err := os.WriteFile(zipPath, []byte("PK\x03\x04rest"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := validateZipFile(zipPath); err != nil {
		t.Errorf("valid zip rejected: %v", err)
	}

	if err := validateGzipFile(zipPath); err == nil {
		t.Error("zip content accepted as gzip")
	}
	if err := validateZipFile(gz); err == nil {
		t.Error("gzip content accepted as zip")
	}

	if err := validateUIArchive("https://example.com/ui.tgz")(gz); err != nil {
		t.Errorf("tgz validator rejected gzip: %v", err)
	}
	if err := validateUIArchive("https://example.com/ui.zip")(zipPath); err != nil {
		t.Errorf("zip validator rejected zip: %v", err)
	}
}
