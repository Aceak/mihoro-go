package mihoro

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"mihoro-go/internal/bin"
	"mihoro-go/internal/config"
	"mihoro-go/internal/proxy"
	"mihoro-go/internal/systemctl"
	"mihoro-go/internal/ui"
	"mihoro-go/internal/utils"
)

// InitOptions holds the flags for `mihoro init`.
type InitOptions struct {
	Force     bool   // Re-download all artifacts even if they already exist
	Arch      string // Override architecture detection (e.g. "amd64", "arm64")
	Yes       bool   // Non-interactive mode: fail if required fields are missing
	System    bool   // Install as system-level service (requires root)
	UserAgent string // Custom User-Agent header for HTTP requests
	Subscribe string // Remote subscription URL (bypasses interactive prompt)
	AllowLan  bool   // Enable LAN access (sets allow_lan in mihomo config.yaml)
}

// --- bootstrap ---

// bootstrapConfig ensures a config file exists and has a remote URL.
// In interactive mode it prompts the user; in --yes mode it returns an error.
// Returns the parsed config, or nil + error if the user cancels (context.Canceled).
func bootstrapConfig(ctx context.Context, configPath string, yes bool) (*config.Config, error) {
	justCreated, err := config.WriteDefaultIfMissing(configPath)
	if err != nil {
		return nil, err
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		c := config.DefaultConfig()
		cfg = &c
	}

	if cfg.RemoteConfigURL == "" {
		if yes {
			return nil, fmt.Errorf("remote_config_url is not set - edit %q or run `mihoro init` interactively", configPath)
		}
		if justCreated {
			fmt.Printf("mihoro: Created default config at %s\n", configPath)
		}
		fmt.Println("mihoro: Enter your remote subscription URL:")
		url, err := promptURL(ctx)
		if err != nil {
			return nil, err
		}
		cfg.RemoteConfigURL = url
		if err := cfg.Save(configPath); err != nil {
			return nil, fmt.Errorf("save config: %w", err)
		}
	}
	return cfg, nil
}

// promptURL reads a subscription URL from stdin. Returns context.Canceled on Ctrl+C.
func promptURL(ctx context.Context) (string, error) {
	fmt.Print("Remote subscription URL: ")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("no input")
	}
	url := strings.TrimSpace(scanner.Text())
	if url == "" {
		return "", fmt.Errorf("URL cannot be empty")
	}
	return url, scanner.Err()
}

// --- RunInit ---

func RunInit(ctx context.Context, client *http.Client, configPath string, opts InitOptions) error {
	configPath = ExpandTilde(configPath)
	cfg, err := bootstrapConfig(ctx, configPath, opts.Yes)
	if err != nil {
		return err
	}

	// Apply optional flag overrides, then save once.
	dirty := false
	if opts.Subscribe != "" {
		cfg.RemoteConfigURL = opts.Subscribe
		dirty = true
	}
	if opts.AllowLan {
		allowLan := true
		cfg.MihomoConfig.AllowLan = &allowLan
		dirty = true
	}
	if opts.UserAgent != "" {
		cfg.MihoroUserAgent = opts.UserAgent
		dirty = true
	}
	if dirty {
		if err := cfg.Save(configPath); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
	}

	if err := validateConfig(cfg); err != nil {
		return err
	}

	if opts.System {
		testFile := "/etc/systemd/system/.mihoro_test"
		if err := os.WriteFile(testFile, []byte{}, 0644); err != nil {
			return fmt.Errorf("--system requires root privileges\n       Try: sudo mihoro init --system")
		}
		_ = os.Remove(testFile)
	}

	m := FromConfig(cfg, opts.System)

	fmt.Println("mihoro: initializing")
	fmt.Printf("  Config:  %s\n", configPath)
	fmt.Printf("  Remote:  %s\n", cfg.RemoteConfigURL)
	if mirror := os.Getenv("MIHORO_GITHUB_MIRROR"); mirror != "" {
		fmt.Printf("  Mirror:  %s\n", mirror)
	}
	fmt.Println()

	force := opts.Force
	arch := opts.Arch

	// --- Phase 1: downloads ---
	fmt.Println("  Downloading components:")

	if _, err := m.EnsureRemoteConfig(ctx, client, force); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	binaryPlan, err := m.PrepareBinary(ctx, client, force, arch)
	if err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if _, err := m.EnsureGeodata(ctx, client, force); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if _, err := m.EnsureUI(ctx, client, force); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// --- Phase 2: install ---
	fmt.Println()
	fmt.Println("  Installing:")

	if binaryPlan.ShouldInstall() {
		if _, err := m.InstallBinary(ctx, binaryPlan.TempFile); err != nil {
			return fmt.Errorf("install: %w", err)
		}
	}

	if _, err := m.EnsureService(ctx); err != nil {
		return fmt.Errorf("service: %w", err)
	}

	if _, err := m.EnsureServiceRunning(ctx); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	fmt.Println()
	if cfg.UI != nil {
		printDashboardURLs(cfg)
	}
	if opts.AllowLan {
		printLanProxyInfo(cfg)
	}
	return nil
}

// --- Init methods on Mihoro ---

func (m *Mihoro) PrepareBinary(ctx context.Context, client *http.Client, force bool, archOverride string) (BinaryPlan, error) {
	if !force && m.binaryUsable() {
		fmt.Printf("  mihomo core %s\n", m.BinaryPath)
		return BinaryPlan{SkipReason: fmt.Sprintf("binary exists at %s", m.BinaryPath)}, nil
	}

	fmt.Fprintf(os.Stderr, "  mihomo core checking version...\033[K\r")
	url, err := bin.ResolveBinaryURL(ctx, client, m.Config, archOverride)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\r  mihomo core \033[K\n")
		return BinaryPlan{}, fmt.Errorf("resolve binary URL: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "mihoro-binary-*")
	if err != nil {
		return BinaryPlan{}, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()

	if err := utils.DownloadFile(ctx, client, url, tmpPath, m.Config.MihoroUserAgent, "  mihomo core"); err != nil {
		_ = os.Remove(tmpPath)
		return BinaryPlan{}, fmt.Errorf("download binary: %w", err)
	}

	return BinaryPlan{TempFile: tmpPath}, nil
}

func (m *Mihoro) InstallBinary(ctx context.Context, tempFilePath string) (StageStatus, error) {
	if _, err := os.Stat(m.BinaryPath); err == nil {
		fmt.Println("  install     Stopping mihomo.service before overwriting...")
		sctl := systemctl.New(m.SystemdScope)
		_ = sctl.Stop("mihomo.service")
	}

	if err := utils.ExtractGzip(tempFilePath, m.BinaryPath, "   "); err != nil {
		return StageFailed, fmt.Errorf("extract binary: %w", err)
	}
	defer func() { _ = os.Remove(tempFilePath) }()

	if err := os.Chmod(m.BinaryPath, 0755); err != nil {
		return StageFailed, fmt.Errorf("chmod binary: %w", err)
	}

	fmt.Printf("  install     Installed to %s\n", m.BinaryPath)
	return StageInstalled, nil
}

func (m *Mihoro) EnsureRemoteConfig(ctx context.Context, client *http.Client, force bool) (StageStatus, error) {
	if !force {
		if _, err := os.Stat(m.ConfigPath); err == nil {
			changed, err := config.ApplyOverride(m.ConfigPath, &m.Config.MihomoConfig)
			if err != nil {
				return StageFailed, fmt.Errorf("apply override: %w", err)
			}
			if changed {
				fmt.Println("  subscribe   Updated overrides")
				return StageInstalled, nil
			}
			fmt.Println("  subscribe   Already current")
			return StageSkipped, nil
		}
	}

	if err := utils.DownloadFile(ctx, client, m.Config.RemoteConfigURL, m.ConfigPath, m.Config.MihoroUserAgent, "  subscribe  "); err != nil {
		return StageFailed, err
	}

	if err := utils.TryDecodeBase64InPlace(m.ConfigPath); err != nil {
		return StageFailed, fmt.Errorf("decode config: %w", err)
	}

	if _, err := config.ApplyOverride(m.ConfigPath, &m.Config.MihomoConfig); err != nil {
		return StageFailed, fmt.Errorf("apply override: %w", err)
	}

	return StageInstalled, nil
}

func (m *Mihoro) EnsureGeodata(ctx context.Context, client *http.Client, force bool) (StageStatus, error) {
	geox := m.Config.MihomoConfig.GeoxUrl
	if geox == nil {
		return StageSkipped, nil
	}

	geodataMode := false
	if m.Config.MihomoConfig.GeodataMode != nil {
		geodataMode = *m.Config.MihomoConfig.GeodataMode
	}

	if geodataMode {
		geoipPath := m.ConfigRoot + "/geoip.dat"
		geositePath := m.ConfigRoot + "/geosite.dat"
		if !force {
			_, err1 := os.Stat(geoipPath)
			_, err2 := os.Stat(geositePath)
			if err1 == nil && err2 == nil {
				fmt.Println("  geodata     Already present")
				return StageSkipped, nil
			}
		}
		if force || !fileExists(geoipPath) {
			if err := utils.DownloadFile(ctx, client, geox.Geoip, geoipPath, m.Config.MihoroUserAgent, "  geodata    "); err != nil {
				return StageFailed, fmt.Errorf("download geoip.dat: %w", err)
			}
		}
		if force || !fileExists(geositePath) {
			if err := utils.DownloadFile(ctx, client, geox.Geosite, geositePath, m.Config.MihoroUserAgent, "  geodata    "); err != nil {
				return StageFailed, fmt.Errorf("download geosite.dat: %w", err)
			}
		}
	} else {
		mmdbPath := m.ConfigRoot + "/country.mmdb"
		if !force {
			if _, err := os.Stat(mmdbPath); err == nil {
				fmt.Println("  geodata     Already present")
				return StageSkipped, nil
			}
		}
		if err := utils.DownloadFile(ctx, client, geox.Mmdb, mmdbPath, m.Config.MihoroUserAgent, "  geodata    "); err != nil {
			return StageFailed, fmt.Errorf("download country.mmdb: %w", err)
		}
	}

	return StageInstalled, nil
}

func (m *Mihoro) EnsureUI(ctx context.Context, client *http.Client, force bool) (StageStatus, error) {
	uiCfg := m.Config.UI
	if uiCfg == nil {
		return StageSkipped, nil
	}

	externalUI := m.Config.MihomoConfig.ExternalUI
	if externalUI == nil || *externalUI == "" {
		return StageSkipped, nil
	}

	targetDir := resolveExternalUIPath(m.ConfigRoot, *externalUI)

	if !force {
		if _, err := os.Stat(targetDir + "/index.html"); err == nil {
			fmt.Println("  web ui      Already installed")
			return StageSkipped, nil
		}
	}

	if err := ui.InstallUI(ctx, client, *uiCfg, targetDir, m.Config.MihoroUserAgent, "   "); err != nil {
		return StageFailed, fmt.Errorf("install ui: %w", err)
	}

	return StageInstalled, nil
}

func (m *Mihoro) EnsureService(ctx context.Context) (StageStatus, error) {
	serviceContent := systemctl.RenderServiceString(m.BinaryPath, m.ConfigRoot, m.SystemdScope)

	existing, err := os.ReadFile(m.ServicePath)
	if err == nil && string(existing) == serviceContent {
		fmt.Println("  systemd     Already configured")
		return StageSkipped, nil
	}

	if err := os.MkdirAll(strings.TrimSuffix(m.ServicePath, "/mihomo.service"), 0755); err != nil {
		return StageFailed, fmt.Errorf("create service dir: %w", err)
	}
	if err := os.WriteFile(m.ServicePath, []byte(serviceContent), 0644); err != nil {
		return StageFailed, fmt.Errorf("write service file: %w", err)
	}

	sctl := systemctl.New(m.SystemdScope)
	if err := sctl.DaemonReload(); err != nil {
		return StageFailed, fmt.Errorf("daemon-reload: %w", err)
	}

	fmt.Printf("  systemd     Created %s\n", m.ServicePath)
	return StageInstalled, nil
}

func (m *Mihoro) EnsureServiceRunning(ctx context.Context) (StageStatus, error) {
	sctl := systemctl.New(m.SystemdScope)

	if sctl.IsActive("mihomo.service") && sctl.IsEnabled("mihomo.service") {
		fmt.Println("  start       Already running")
		return StageSkipped, nil
	}

	if !sctl.IsEnabled("mihomo.service") {
		if err := sctl.Enable("mihomo.service"); err != nil {
			return StageFailed, fmt.Errorf("enable: %w", err)
		}
	}
	if !sctl.IsActive("mihomo.service") {
		if err := sctl.Start("mihomo.service"); err != nil {
			return StageFailed, fmt.Errorf("start: %w", err)
		}
	}
	fmt.Println("  start       Started")
	return StageInstalled, nil
}

// --- helpers ---

func (m *Mihoro) binaryUsable() bool {
	if _, err := os.Stat(m.BinaryPath); os.IsNotExist(err) {
		return false
	}
	_, err := m.InstalledVersion()
	return err == nil
}

func validateConfig(cfg *config.Config) error {
	for _, f := range []struct{ name, val string }{
		{"remote_config_url", cfg.RemoteConfigURL},
		{"mihomo_binary_path", cfg.MihomoBinaryPath},
		{"mihomo_config_root", cfg.MihomoConfigRoot},
		{"user_systemd_root", cfg.UserSystemdRoot},
	} {
		if f.val == "" {
			return fmt.Errorf("%q is undefined", f.name)
		}
	}
	return nil
}

func resolveExternalUIPath(configRoot, externalUI string) string {
	if strings.HasPrefix(externalUI, "/") {
		return externalUI
	}
	return configRoot + "/" + externalUI
}

// --- LAN proxy info ---

func printLanProxyInfo(cfg *config.Config) {
	ip := proxy.LocalIP()
	shell := proxy.DetectShell()
	port, socksPort := proxy.GetPorts(cfg.MihomoConfig)

	fmt.Println("  LAN proxy enabled:")
	fmt.Printf("    %s\n", proxy.ExportCmd(shell, ip, port, socksPort))
	fmt.Println("    Use `mihoro proxy export-lan` to regenerate this command")
}
