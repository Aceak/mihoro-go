package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.UI == nil || *cfg.UI != UiMetacubexd {
		t.Errorf("default ui = %v, want metacubexd", cfg.UI)
	}
	if cfg.MihomoChannel != ChannelStable {
		t.Errorf("default channel = %s, want stable", cfg.MihomoChannel)
	}
	if cfg.MihomoBinaryPath != "~/.local/bin/mihomo" {
		t.Errorf("default binary path = %s", cfg.MihomoBinaryPath)
	}
	if cfg.MihomoConfig.Port == nil || *cfg.MihomoConfig.Port != 7891 {
		t.Errorf("default port = %v, want 7891", cfg.MihomoConfig.Port)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.toml")

	cfg := DefaultConfig()

	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save() = %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil config")
	}
	if *loaded.UI != UiMetacubexd {
		t.Errorf("UI = %s, want metacubexd", *loaded.UI)
	}
}

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path.toml")
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if cfg != nil {
		t.Error("expected nil config for missing file")
	}
}

func TestWriteDefaultIfMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.toml")

	created, err := WriteDefaultIfMissing(path)
	if err != nil {
		t.Fatalf("WriteDefaultIfMissing() = %v", err)
	}
	if !created {
		t.Error("expected created=true")
	}

	created2, err := WriteDefaultIfMissing(path)
	if err != nil {
		t.Fatalf("2nd WriteDefaultIfMissing() = %v", err)
	}
	if created2 {
		t.Error("expected created=false on second call")
	}
}

func TestParseConfigCreatesDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.toml")

	_, err := ParseConfig(path)
	if err == nil {
		t.Fatal("expected error for missing config")
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected default config file to be created")
	}
}

func TestParseConfigValidatesRequiredFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.toml")

	content := `
mihomo_binary_path = ""
mihomo_config_root = "~/.config/mihomo"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ParseConfig(path)
	if err == nil {
		t.Fatal("expected validation error for missing mihomo_binary_path")
	}
}

func TestParseUi(t *testing.T) {
	tests := []struct {
		input string
		want  Ui
		err   bool
	}{
		{"metacubexd", UiMetacubexd, false},
		{"zashboard", UiZashboard, false},
		{"yacd-meta", UiYacdMeta, false},
		{"custom:https://example.com/ui.tar.gz", Ui("custom:https://example.com/ui.tar.gz"), false},
		{"invalid", "", true},
		{"custom:", "", true},
	}

	for _, tt := range tests {
		got, err := ParseUi(tt.input)
		if tt.err && err == nil {
			t.Errorf("ParseUi(%q) = %s, want error", tt.input, got)
		}
		if !tt.err {
			if err != nil {
				t.Errorf("ParseUi(%q) = error %v, want %s", tt.input, err, tt.want)
			}
			if got != tt.want {
				t.Errorf("ParseUi(%q) = %s, want %s", tt.input, got, tt.want)
			}
		}
	}
}

func TestUiDownloadURL(t *testing.T) {
	tests := []struct {
		ui   Ui
		want string
	}{
		{UiMetacubexd, "https://github.com/MetaCubeX/metacubexd/releases/latest/download/compressed-dist.tgz"},
		{UiZashboard, "https://github.com/Zephyruso/zashboard/releases/latest/download/dist.zip"},
		{UiYacdMeta, "https://github.com/MetaCubeX/Yacd-meta/archive/refs/heads/gh-pages.tar.gz"},
		{Ui("custom:https://example.com/ui.tar.gz"), "https://example.com/ui.tar.gz"},
	}

	for _, tt := range tests {
		got := tt.ui.DownloadURL()
		if got != tt.want {
			t.Errorf("%s.DownloadURL() = %s, want %s", tt.ui, got, tt.want)
		}
	}
}

func TestApplyOverride(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")

	yamlContent := `
port: 8080
socks-port: 8081
mixed-port: 7890
mode: rule
log-level: info
proxies:
  - name: "test"
    type: http
    server: example.com
    port: 443
`
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	override := DefaultMihomoConfig()
	override.Port = ptr(uint16(7891))
	override.SocksPort = ptr(uint16(7892))

	changed, err := ApplyOverride(yamlPath, &override)
	if err != nil {
		t.Fatalf("ApplyOverride() = %v", err)
	}
	if !changed {
		t.Error("expected changed=true")
	}

	data, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strContains(content, "port: 7891") {
		t.Error("expected port 7891 in output")
	}
	if !strContains(content, "socks-port: 7892") {
		t.Error("expected socks-port 7892 in output")
	}
	if !strContains(content, "proxies:") {
		t.Error("expected proxies to be preserved")
	}
}

func TestApplyOverrideSkipsWhenUnchanged(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")

	override := DefaultMihomoConfig()

	var yml MihomoYamlConfig
	yml.Port = ptrIntIfNotNil(override.Port)
	yml.SocksPort = ptrIntIfNotNil(override.SocksPort)
	yml.MixedPort = ptrIntIfNotNil(override.MixedPort)
	yml.RedirPort = ptrIntIfNotNil(override.RedirPort)
	yml.AllowLan = override.AllowLan
	yml.BindAddress = override.BindAddress
	mode := override.Mode
	yml.Mode = &mode
	logLevel := override.LogLevel
	yml.LogLevel = &logLevel
	yml.IPv6 = override.IPv6
	yml.ExternalController = override.ExternalController
	yml.ExternalUI = override.ExternalUI
	yml.Secret = override.Secret
	yml.GeodataMode = override.GeodataMode
	yml.GeoAutoUpdate = override.GeoAutoUpdate
	yml.GeoUpdateInterval = override.GeoUpdateInterval
	yml.GeoxUrl = override.GeoxUrl
	yml.Extra = map[string]any{
		"proxies": []any{
			map[string]any{"name": "test", "type": "http", "server": "example.com", "port": 443},
		},
	}

	rawYaml, err := yaml.Marshal(&yml)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(yamlPath, rawYaml, 0644); err != nil {
		t.Fatal(err)
	}

	changed, err := ApplyOverride(yamlPath, &override)
	if err != nil {
		t.Fatalf("ApplyOverride() = %v", err)
	}
	if changed {
		t.Error("expected changed=false for already-matching YAML")
	}
}

func TestApplyOverridePreservesExtraFields(t *testing.T) {
	dir := t.TempDir()
	yamlPath := dir + "/config.yaml"
	yamlContent := `
port: 8080
socks-port: 8081
proxies:
  - name: "my-proxy"
    type: ss
    server: example.com
    port: 8388
proxy-groups:
  - name: "Auto"
    type: url-test
rules:
  - DOMAIN-SUFFIX,google.com,Auto
`
	_ = os.WriteFile(yamlPath, []byte(yamlContent), 0644)

	override := DefaultMihomoConfig()
	override.Port = ptr(uint16(9999))
	override.SocksPort = ptr(uint16(9998))

	_, err := ApplyOverride(yamlPath, &override)
	if err != nil {
		t.Fatalf("ApplyOverride() = %v", err)
	}

	data, _ := os.ReadFile(yamlPath)
	content := string(data)
	if !strContains(content, "port: 9999") {
		t.Error("port should be overridden")
	}
	if !strContains(content, "my-proxy") {
		t.Error("proxies should be preserved")
	}
	if !strContains(content, "proxy-groups") {
		t.Error("proxy-groups should be preserved")
	}
	if !strContains(content, "rules") {
		t.Error("rules should be preserved")
	}
}

func TestConfigFullRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/full.toml"

	cfg := DefaultConfig()
	cfg.MihomoChannel = ChannelAlpha
	cfg.MihomoConfig.Port = ptr(uint16(1234))
	cfg.MihomoConfig.SocksPort = ptr(uint16(1235))

	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save() = %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if loaded.MihomoChannel != ChannelAlpha {
		t.Errorf("MihomoChannel mismatch: %s", loaded.MihomoChannel)
	}
	if loaded.MihomoConfig.Port == nil || *loaded.MihomoConfig.Port != 1234 {
		t.Errorf("Port mismatch: %v", loaded.MihomoConfig.Port)
	}
}

func TestGeoxUrlDefault(t *testing.T) {
	cfg := DefaultMihomoConfig()
	if cfg.GeoxUrl == nil {
		t.Fatal("GeoxUrl should have default")
	}
	if !strContains(cfg.GeoxUrl.Geoip, "github.com") {
		t.Error("Geoip URL should contain github.com")
	}
	if !strContains(cfg.GeoxUrl.Geosite, "geosite.dat") {
		t.Error("Geosite URL should contain geosite.dat")
	}
	if !strContains(cfg.GeoxUrl.Mmdb, "country.mmdb") {
		t.Error("Mmdb URL should contain country.mmdb")
	}
}

func TestDefaultMihomoConfigSecure(t *testing.T) {
	cfg := DefaultMihomoConfig()
	if cfg.ExternalController == nil || *cfg.ExternalController != "127.0.0.1:9090" {
		t.Errorf("default external controller = %v, want 127.0.0.1:9090", cfg.ExternalController)
	}
	if cfg.Secret != nil {
		t.Errorf("default secret = %v, want nil", *cfg.Secret)
	}
}

func TestSaveFilePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.toml")

	cfg := DefaultConfig()
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save() = %v", err)
	}
	assertPrivatePerm(t, path)

	// A pre-existing world-readable file must be replaced with mode 0600.
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save() = %v", err)
	}
	assertPrivatePerm(t, path)
}

func TestApplyOverridePermissions(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")

	override := DefaultMihomoConfig()
	rawYaml, err := yaml.Marshal(mihomoYamlFrom(&override))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(yamlPath, rawYaml, 0644); err != nil {
		t.Fatal(err)
	}

	// Unchanged content still goes through the chmod path.
	changed, err := ApplyOverride(yamlPath, &override)
	if err != nil {
		t.Fatalf("ApplyOverride() = %v", err)
	}
	if changed {
		t.Error("expected changed=false for already-matching YAML")
	}
	assertPrivatePerm(t, yamlPath)

	// Changed content path.
	if err := os.Chmod(yamlPath, 0644); err != nil {
		t.Fatal(err)
	}
	override.Port = ptr(uint16(9999))
	changed, err = ApplyOverride(yamlPath, &override)
	if err != nil {
		t.Fatalf("ApplyOverride() = %v", err)
	}
	if !changed {
		t.Error("expected changed=true after modifying port")
	}
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strContains(string(data), "port: 9999") {
		t.Error("override changes were not written to the file")
	}
	assertPrivatePerm(t, yamlPath)
}

// mihomoYamlFrom mirrors override fields onto a YAML struct the same way
// ApplyOverride does, for building "already matching" fixtures.
func mihomoYamlFrom(override *MihomoConfig) *MihomoYamlConfig {
	var yml MihomoYamlConfig
	yml.Port = ptrIntIfNotNil(override.Port)
	yml.SocksPort = ptrIntIfNotNil(override.SocksPort)
	yml.MixedPort = ptrIntIfNotNil(override.MixedPort)
	yml.RedirPort = ptrIntIfNotNil(override.RedirPort)
	yml.AllowLan = override.AllowLan
	yml.BindAddress = override.BindAddress
	mode := override.Mode
	yml.Mode = &mode
	logLevel := override.LogLevel
	yml.LogLevel = &logLevel
	yml.IPv6 = override.IPv6
	yml.ExternalController = override.ExternalController
	yml.ExternalUI = override.ExternalUI
	yml.Secret = override.Secret
	yml.GeodataMode = override.GeodataMode
	yml.GeoAutoUpdate = override.GeoAutoUpdate
	yml.GeoUpdateInterval = override.GeoUpdateInterval
	yml.GeoxUrl = override.GeoxUrl
	return &yml
}

func assertPrivatePerm(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("%s permissions = %o, want 600", path, perm)
	}
}

func strContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
