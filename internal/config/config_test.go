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
	if cfg.MihomoConfig.Port != 7891 {
		t.Errorf("default port = %d, want 7891", cfg.MihomoConfig.Port)
	}
	if cfg.AutoUpdateInterval != 12 {
		t.Errorf("default auto update interval = %d, want 12", cfg.AutoUpdateInterval)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.toml")

	cfg := DefaultConfig()
	cfg.RemoteConfigURL = "http://example.com/config.yaml"

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
	if loaded.RemoteConfigURL != "http://example.com/config.yaml" {
		t.Errorf("RemoteConfigURL = %s, want url", loaded.RemoteConfigURL)
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
mihomo_binary_path = "~/.local/bin/mihomo"
mihomo_config_root = "~/.config/mihomo"
user_systemd_root = "~/.config/systemd/user"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ParseConfig(path)
	if err == nil {
		t.Fatal("expected validation error for missing remote_config_url")
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
	override.Port = 7891
	override.SocksPort = 7892

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
	port := int(override.Port)
	yml.Port = &port
	socksPort := int(override.SocksPort)
	yml.SocksPort = &socksPort
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
	override.Port = 9999
	override.SocksPort = 9998

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

func TestApplyOverrideWithNilOptionals(t *testing.T) {
	dir := t.TempDir()
	yamlPath := dir + "/config.yaml"
	yamlContent := `
port: 8080
socks-port: 8081
mixed-port: 7888
`
	_ = os.WriteFile(yamlPath, []byte(yamlContent), 0644)

	// Override with nil MixedPort → should remove mixed-port from yaml
	override := DefaultMihomoConfig()
	override.MixedPort = nil

	_, err := ApplyOverride(yamlPath, &override)
	if err != nil {
		t.Fatalf("ApplyOverride() = %v", err)
	}

	data, _ := os.ReadFile(yamlPath)
	content := string(data)
	if strContains(content, "mixed-port") {
		t.Error("mixed-port should be removed when nil in override")
	}
}

func TestApplyOverrideNoChangeNoWrite(t *testing.T) {
	dir := t.TempDir()
	yamlPath := dir + "/config.yaml"

	override := DefaultMihomoConfig()

	// Write initial YAML
	if err := os.WriteFile(yamlPath, []byte("port: 7891\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Apply override — should change
	changed, err := ApplyOverride(yamlPath, &override)
	if err != nil {
		t.Fatalf("ApplyOverride() = %v", err)
	}
	if !changed {
		t.Error("first apply should report changed")
	}
	// Apply again — should be unchanged
	changed, err = ApplyOverride(yamlPath, &override)
	if err != nil {
		t.Fatalf("ApplyOverride() 2nd = %v", err)
	}
	if changed {
		t.Error("second apply should report unchanged")
	}
}

func TestConfigFullRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/full.toml"

	cfg := DefaultConfig()
	cfg.RemoteConfigURL = "https://sub.example.com/config.yaml"
	cfg.MihomoChannel = ChannelAlpha
	cfg.AutoUpdateInterval = 6
	cfg.MihomoConfig.Port = 1234
	cfg.MihomoConfig.SocksPort = 1235

	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save() = %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if loaded.RemoteConfigURL != cfg.RemoteConfigURL {
		t.Errorf("RemoteConfigURL mismatch")
	}
	if loaded.MihomoChannel != ChannelAlpha {
		t.Errorf("MihomoChannel mismatch: %s", loaded.MihomoChannel)
	}
	if loaded.AutoUpdateInterval != 6 {
		t.Errorf("AutoUpdateInterval mismatch: %d", loaded.AutoUpdateInterval)
	}
	if loaded.MihomoConfig.Port != 1234 {
		t.Errorf("Port mismatch: %d", loaded.MihomoConfig.Port)
	}
}

func TestParseUiEdgeCases(t *testing.T) {
	// Empty
	_, err := ParseUi("")
	if err == nil {
		t.Error("empty should error")
	}
	// Unknown
	_, err = ParseUi("unknown-ui-name")
	if err == nil {
		t.Error("unknown ui should error")
	}
	// custom with empty URL
	_, err = ParseUi("custom:")
	if err == nil {
		t.Error("custom: with empty URL should error")
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

func strContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
