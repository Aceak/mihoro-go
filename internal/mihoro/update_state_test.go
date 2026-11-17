package mihoro

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	st := newUpdateState()
	st.UI["metacubexd"] = "v1.274.6"
	st.UI["zashboard"] = "v3.30.0"
	if err := saveUpdateState(dir, st); err != nil {
		t.Fatalf("saveUpdateState() = %v", err)
	}

	loaded, err := loadUpdateState(dir)
	if err != nil {
		t.Fatalf("loadUpdateState() = %v", err)
	}
	if loaded.UI["metacubexd"] != "v1.274.6" || loaded.UI["zashboard"] != "v3.30.0" {
		t.Errorf("loaded state = %+v", loaded.UI)
	}
}

func TestLoadUpdateStateMissing(t *testing.T) {
	st, err := loadUpdateState(t.TempDir())
	if err != nil {
		t.Fatalf("loadUpdateState() = %v", err)
	}
	if st == nil || st.UI == nil {
		t.Fatal("expected an initialized empty state")
	}
}

func TestLoadUpdateStateCorrupt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(updateStatePath(dir), []byte("not = valid = toml"), 0644); err != nil {
		t.Fatal(err)
	}
	st, err := loadUpdateState(dir)
	if err == nil {
		t.Fatal("expected a parse error")
	}
	if st == nil {
		t.Fatal("expected a usable empty state alongside the error")
	}
}

func TestFileSHA256AndDigestHex(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data")
	if err := os.WriteFile(path, []byte("abc"), 0644); err != nil {
		t.Fatal(err)
	}
	// sha256("abc")
	const want = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	got, err := fileSHA256(path)
	if err != nil {
		t.Fatalf("fileSHA256() = %v", err)
	}
	if got != want {
		t.Errorf("fileSHA256() = %s, want %s", got, want)
	}
	if digestHex(" sha256:"+want+" ") != want {
		t.Errorf("digestHex() = %q", digestHex(" sha256:"+want+" "))
	}
}

func TestIsMetaRulesDatURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://github.com/MetaCubeX/meta-rules-dat/releases/latest/download/country.mmdb", true},
		{"https://ghfast.top/https://github.com/MetaCubeX/meta-rules-dat/releases/latest/download/country.mmdb", true},
		{"https://github.com/other/repo/releases/latest/download/country.mmdb", false},
		{"https://example.com/country.mmdb", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isMetaRulesDatURL(tt.url); got != tt.want {
			t.Errorf("isMetaRulesDatURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}
