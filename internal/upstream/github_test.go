package upstream

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func withTestAPI(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	old := APIBase
	APIBase = srv.URL
	t.Cleanup(func() {
		APIBase = old
		srv.Close()
	})
}

func TestLatestRelease(t *testing.T) {
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/MetaCubeX/meta-rules-dat/releases/latest" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"tag_name": "latest",
			"assets": [
				{"name": "country.mmdb", "size": 123, "digest": "sha256:abcd"},
				{"name": "geoip.dat", "size": 456}
			]
		}`)
	})

	rel, err := LatestRelease(context.Background(), nil, "MetaCubeX", "meta-rules-dat")
	if err != nil {
		t.Fatalf("LatestRelease() = %v", err)
	}
	if rel.Tag != "latest" {
		t.Errorf("tag = %q, want latest", rel.Tag)
	}
	asset := rel.AssetByName("country.mmdb")
	if asset == nil {
		t.Fatal("country.mmdb asset missing")
	}
	if asset.Size != 123 || asset.Digest != "sha256:abcd" {
		t.Errorf("asset = %+v", *asset)
	}
	if rel.AssetByName("missing") != nil {
		t.Error("unexpected asset found")
	}
}

func TestLatestReleaseHTTPError(t *testing.T) {
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})

	if _, err := LatestRelease(context.Background(), nil, "owner", "repo"); err == nil {
		t.Fatal("expected an error for HTTP 404")
	}
}

func TestLatestReleaseEmptyTag(t *testing.T) {
	withTestAPI(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"assets": []}`)
	})

	if _, err := LatestRelease(context.Background(), nil, "owner", "repo"); err == nil {
		t.Fatal("expected an error for a missing tag")
	}
}
