package mihoro

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreparedGeodataApply(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "country.mmdb")
	if err := os.WriteFile(dest, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	tmp, err := os.CreateTemp(dir, ".country.mmdb.update-*")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmp.WriteString("new"); err != nil {
		t.Fatal(err)
	}
	_ = tmp.Close()

	p := &preparedGeodata{files: []geodataFile{{tmpPath: tmp.Name(), destPath: dest}}}
	if err := p.Apply(); err != nil {
		t.Fatalf("Apply() = %v", err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Errorf("content = %q, want new", data)
	}
	if _, err := os.Stat(tmp.Name()); !os.IsNotExist(err) {
		t.Errorf("temp file still exists, stat err = %v", err)
	}
}

func TestPreparedGeodataDiscard(t *testing.T) {
	dir := t.TempDir()
	tmp := filepath.Join(dir, ".country.mmdb.update-1")
	if err := os.WriteFile(tmp, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}

	p := &preparedGeodata{files: []geodataFile{{tmpPath: tmp, destPath: filepath.Join(dir, "country.mmdb")}}}
	p.Discard()
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Errorf("temp file still exists after Discard, stat err = %v", err)
	}
}

func TestPreparedUIApply(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "ui")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "index.html"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	base := filepath.Join(root, ".mihoro-ui-staging")
	src := filepath.Join(base, "extracted")
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "index.html"), []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}

	cfgDir := t.TempDir()
	p := &preparedUI{
		m:         &Mihoro{ConfigDir: cfgDir},
		baseDir:   base,
		srcDir:    src,
		targetDir: target,
		uiKey:     "metacubexd",
		tag:       "v1.2.3",
	}
	if err := p.Apply(); err != nil {
		t.Fatalf("Apply() = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(target, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Errorf("index.html = %q, want new", data)
	}

	st, err := loadUpdateState(cfgDir)
	if err != nil {
		t.Fatalf("loadUpdateState() = %v", err)
	}
	if st.UI["metacubexd"] != "v1.2.3" {
		t.Errorf("state tag = %q, want v1.2.3", st.UI["metacubexd"])
	}

	p.Discard()
	if _, err := os.Stat(base); !os.IsNotExist(err) {
		t.Errorf("staging dir still exists after Discard, stat err = %v", err)
	}

	// The old directory must not be left behind as a backup.
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".mihoro-ui-old-") {
			t.Errorf("leftover backup dir: %s", e.Name())
		}
	}
}

func TestPreparedUIApplyRestoresBackupOnFailure(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "ui")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "index.html"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	p := &preparedUI{
		m:         &Mihoro{ConfigDir: t.TempDir()},
		baseDir:   filepath.Join(root, ".mihoro-ui-staging"),
		srcDir:    filepath.Join(root, "does-not-exist"),
		targetDir: target,
		uiKey:     "metacubexd",
		tag:       "v1.2.3",
	}
	if err := p.Apply(); err == nil {
		t.Fatal("expected Apply to fail when the staged directory is missing")
	}

	data, err := os.ReadFile(filepath.Join(target, "index.html"))
	if err != nil {
		t.Fatalf("old ui was not restored: %v", err)
	}
	if string(data) != "old" {
		t.Errorf("index.html = %q, want the old content", data)
	}
}

func TestPreparedUIResetsCorruptState(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "ui")
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}

	cfgDir := t.TempDir()
	if err := os.WriteFile(updateStatePath(cfgDir), []byte("broken = = toml"), 0644); err != nil {
		t.Fatal(err)
	}

	base := filepath.Join(root, ".mihoro-ui-staging")
	src := filepath.Join(base, "extracted")
	if err := os.MkdirAll(src, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "index.html"), []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}

	p := &preparedUI{
		m:         &Mihoro{ConfigDir: cfgDir},
		baseDir:   base,
		srcDir:    src,
		targetDir: target,
		uiKey:     "metacubexd",
		tag:       "v1.2.3",
	}
	if err := p.Apply(); err != nil {
		t.Fatalf("Apply() = %v", err)
	}
	st, err := loadUpdateState(cfgDir)
	if err != nil {
		t.Fatalf("state was not repaired: %v", err)
	}
	if st.UI["metacubexd"] != "v1.2.3" {
		t.Errorf("state tag = %q, want v1.2.3", st.UI["metacubexd"])
	}
}
