package mihoro

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mihoro-go/internal/config"
)

func TestResolveSecret(t *testing.T) {
	existing := "keep-me"

	tests := []struct {
		name       string
		current    *string
		noSecret   bool
		fresh      bool
		line       string
		ok         bool
		wantSource secretSource
		wantSecret string
		wantNil    bool
		wantErr    bool
	}{
		{"existing kept", &existing, false, false, "", true, secretKept, "keep-me", false, false},
		{"existing kept ignores no-secret", &existing, true, true, "", false, secretKept, "keep-me", false, false},
		{"no-secret flag", nil, true, true, "", true, secretDisabled, "", true, false},
		{"typed value", nil, false, true, "mysecret", true, secretInput, "mysecret", false, false},
		{"empty on fresh generates", nil, false, true, "", true, secretGenerated, "", false, false},
		{"eof on fresh generates", nil, false, true, "", false, secretGenerated, "", false, false},
		{"eof on existing install skips", nil, false, false, "", false, secretDisabled, "", true, false},
		{"blank input generates", nil, false, true, "   ", true, secretGenerated, "", false, false},
		{"padded value rejected", nil, false, true, " hunter2 ", true, secretDisabled, "", true, true},
		{"whitespace rejected", nil, false, true, "a b", true, secretDisabled, "", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, source, err := resolveSecret(tt.current, tt.noSecret, tt.fresh, tt.line, tt.ok)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveSecret() = %v", err)
			}
			if source != tt.wantSource {
				t.Errorf("source = %d, want %d", source, tt.wantSource)
			}
			if tt.wantNil {
				if got != nil {
					t.Errorf("secret = %q, want nil", *got)
				}
				return
			}
			if got == nil {
				t.Fatal("secret is nil")
			}
			if tt.wantSource == secretGenerated {
				return // generated values have random content, checked elsewhere
			}
			if *got != tt.wantSecret {
				t.Errorf("secret = %q, want %q", *got, tt.wantSecret)
			}
		})
	}
}

func TestDashboardController(t *testing.T) {
	ptrStr := func(s string) *string { return &s }

	tests := []struct {
		name      string
		current   *string
		exposeLan bool
		want      string
	}{
		{"nil, expose", nil, true, "0.0.0.0:9090"},
		{"nil, local", nil, false, "127.0.0.1:9090"},
		{"loopback, expose", ptrStr("127.0.0.1:9090"), true, "0.0.0.0:9090"},
		{"loopback, local", ptrStr("127.0.0.1:9090"), false, "127.0.0.1:9090"},
		{"wildcard, expose keeps value", ptrStr("0.0.0.0:9090"), true, "0.0.0.0:9090"},
		{"wildcard, local", ptrStr("0.0.0.0:9090"), false, "127.0.0.1:9090"},
		{"custom port preserved", ptrStr("127.0.0.1:19090"), true, "0.0.0.0:19090"},
		{"custom ip kept when local", ptrStr("192.168.1.5:9090"), false, "192.168.1.5:9090"},
		{"custom ip replaced when exposed", ptrStr("192.168.1.5:9090"), true, "0.0.0.0:9090"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dashboardController(tt.current, tt.exposeLan); got != tt.want {
				t.Errorf("dashboardController(%v, %v) = %q, want %q", tt.current, tt.exposeLan, got, tt.want)
			}
		})
	}
}

func TestSetSecretAndReapplyOverrides(t *testing.T) {
	mihoroDir := t.TempDir()
	configRoot := t.TempDir()
	subDir := filepath.Join(mihoroDir, "subscriptions")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	cfg.MihomoConfigRoot = configRoot
	if err := cfg.Save(filepath.Join(mihoroDir, "config.toml")); err != nil {
		t.Fatal(err)
	}

	subName := "test-sub"
	subYaml := "mixed-port: 7890\nproxies:\n  - name: \"p1\"\n    type: http\n    server: example.com\n    port: 443\n"
	if err := os.WriteFile(filepath.Join(subDir, subName+".yaml"), []byte(subYaml), 0644); err != nil {
		t.Fatal(err)
	}

	subs := &config.SubscriptionsFile{
		ActiveSubscription: subName,
		Subscriptions:      []config.Subscription{{Name: subName}},
	}
	m := FromConfig(&cfg, subs, mihoroDir)

	secret := "test-secret-value"
	if err := m.SetSecret(&secret); err != nil {
		t.Fatalf("SetSecret() = %v", err)
	}
	if err := m.ReapplyOverrides(); err != nil {
		t.Fatalf("ReapplyOverrides() = %v", err)
	}

	// config.yaml is rendered from the subscription and carries the secret.
	yamlPath := filepath.Join(configRoot, "config.yaml")
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "secret: test-secret-value") {
		t.Errorf("config.yaml missing secret:\n%s", content)
	}
	if !strings.Contains(content, "proxies:") {
		t.Error("config.yaml should preserve proxies")
	}

	// config.toml persists the secret.
	tomlData, err := os.ReadFile(filepath.Join(mihoroDir, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(tomlData), "test-secret-value") {
		t.Error("config.toml missing secret")
	}
}

func TestReapplyOverridesFallback(t *testing.T) {
	mihoroDir := t.TempDir()
	configRoot := t.TempDir()

	cfg := config.DefaultConfig()
	cfg.MihomoConfigRoot = configRoot
	if err := cfg.Save(filepath.Join(mihoroDir, "config.toml")); err != nil {
		t.Fatal(err)
	}

	yamlPath := filepath.Join(configRoot, "config.yaml")
	if err := os.WriteFile(yamlPath, []byte("port: 7890\nproxies:\n  - name: \"p1\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// No subscription: the existing config.yaml is updated in place.
	m := FromConfig(&cfg, &config.SubscriptionsFile{}, mihoroDir)

	secret := "fallback-secret"
	if err := m.SetSecret(&secret); err != nil {
		t.Fatal(err)
	}
	if err := m.ReapplyOverrides(); err != nil {
		t.Fatalf("ReapplyOverrides() = %v", err)
	}

	data, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "fallback-secret") {
		t.Error("in-place override should add the secret")
	}
	if !strings.Contains(string(data), "p1") {
		t.Error("in-place override should preserve proxies")
	}
}

func TestReapplyOverridesMissingConfigYaml(t *testing.T) {
	mihoroDir := t.TempDir()
	configRoot := t.TempDir()

	cfg := config.DefaultConfig()
	cfg.MihomoConfigRoot = configRoot
	if err := cfg.Save(filepath.Join(mihoroDir, "config.toml")); err != nil {
		t.Fatal(err)
	}

	m := FromConfig(&cfg, &config.SubscriptionsFile{}, mihoroDir)
	if err := m.ReapplyOverrides(); err == nil {
		t.Error("expected error when neither subscription nor config.yaml exists")
	}
}

func TestAccessNoticeLines(t *testing.T) {
	secret := "s3cr3t-9z"
	required := "API secret: required (change with `mihoro secret reset`)"
	unprotected := "warning: no API secret - the dashboard/API is unprotected (run `mihoro secret reset`)"
	exposeHint := "To expose the dashboard on your LAN: mihoro init --force --dashboard-lan"

	tests := []struct {
		name       string
		controller string
		secret     *string
		want       []string
	}{
		{
			"loopback with secret",
			"127.0.0.1:9090", &secret,
			[]string{"API access: local only (127.0.0.1)", exposeHint, required},
		},
		{
			"loopback without secret",
			"127.0.0.1:9090", nil,
			[]string{"API access: local only (127.0.0.1)", exposeHint, unprotected},
		},
		{
			"ipv6 loopback shows its own address",
			"[::1]:9090", &secret,
			[]string{"API access: local only (::1)", exposeHint, required},
		},
		{
			"wildcard with secret",
			"0.0.0.0:9090", &secret,
			[]string{"API access: all interfaces (reachable from the LAN)", required},
		},
		{
			"wildcard without secret",
			"0.0.0.0:9090", nil,
			[]string{"API access: all interfaces (reachable from the LAN)", unprotected},
		},
		{
			"custom binding with secret",
			"10.0.0.1:9090", &secret,
			[]string{"API access: 10.0.0.1:9090", required},
		},
		{
			"empty controller",
			"", &secret,
			[]string{"API access: not configured", required},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.MihomoConfig.ExternalController = &tt.controller
			cfg.MihomoConfig.Secret = tt.secret

			lines := accessNoticeLines(&cfg)
			if len(lines) != len(tt.want) {
				t.Fatalf("got %d lines, want %d: %v", len(lines), len(tt.want), lines)
			}
			for i := range tt.want {
				if lines[i] != tt.want[i] {
					t.Errorf("line %d =\n  %q\nwant\n  %q", i, lines[i], tt.want[i])
				}
				if tt.secret != nil && strings.Contains(lines[i], *tt.secret) {
					t.Errorf("notice must not contain the secret value: %q", lines[i])
				}
			}
		})
	}
}
