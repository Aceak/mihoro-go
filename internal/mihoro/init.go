package mihoro

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"mihoro-go/internal/bin"
	"mihoro-go/internal/config"
	"mihoro-go/internal/proxy"
	"mihoro-go/internal/systemctl"
	"mihoro-go/internal/utils"
)

// InitOptions holds the flags for `mihoro init`.
type InitOptions struct {
	Force    bool
	Arch     string
	AllowLan bool
}

func bootstrapConfig(mihoroDir string) (*config.Config, *config.SubscriptionsFile, error) {
	cfg, sf, err := loadOrCreateConfig(mihoroDir)
	if err != nil {
		return nil, nil, err
	}

	if len(sf.Subscriptions) > 0 {
		if active := sf.Active(); active != nil {
			lastUpdate := "-"
			if active.LastUpdate != "" {
				lastUpdate = active.LastUpdate[:10] + " " + active.LastUpdate[11:16]
			}
			stat := "never"
			switch active.LastStatus {
			case "success":
				stat = fmt.Sprintf("OK (%dKB)", active.LastSize/1024)
			case "failed":
				stat = active.LastError
			}
			fmt.Printf("active subscription: %s  last update: %s  status: %s\n", active.Name, lastUpdate, stat)
		}
	} else {
		return nil, nil, fmt.Errorf("no subscriptions configured. Use 'mihoro sub add' to add one")
	}

	return cfg, sf, nil
}

func loadOrCreateConfig(mihoroDir string) (*config.Config, *config.SubscriptionsFile, error) {
	if err := os.MkdirAll(mihoroDir, 0755); err != nil {
		return nil, nil, fmt.Errorf("create config dir: %w", err)
	}

	cfgPath := ConfigPath(mihoroDir)
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, nil, err
	}
	if cfg == nil {
		c := config.DefaultConfig()
		cfg = &c
		if err := cfg.Save(cfgPath); err != nil {
			return nil, nil, err
		}
	}

	sf, err := config.LoadSubscriptions(mihoroDir)
	if err != nil {
		return nil, nil, err
	}

	return cfg, sf, nil
}

func promptBool(question, defaultValue string) bool {
	var def string
	if strings.EqualFold(defaultValue, "y") {
		def = "Y/n"
	} else {
		def = "y/N"
	}
	fmt.Printf("%s [%s]: ", question, def)

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return strings.EqualFold(defaultValue, "y")
	}
	val := strings.TrimSpace(scanner.Text())
	if val == "" {
		return strings.EqualFold(defaultValue, "y")
	}
	return strings.EqualFold(val, "y") || strings.EqualFold(val, "yes")
}

func promptString(question, defaultValue string) string {
	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", question, defaultValue)
	} else {
		fmt.Printf("%s: ", question)
	}
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return defaultValue
	}
	val := strings.TrimSpace(scanner.Text())
	if val == "" {
		return defaultValue
	}
	return val
}

func RunInit(ctx context.Context, client *http.Client, mihoroDir string, opts InitOptions, mirrorFlag string) error {
	mihoroDir = ExpandTilde(mihoroDir)
	mirror := mirrorFlag

	fmt.Printf("config dir: %s\n", mihoroDir)

	cfg, sf, err := bootstrapConfig(mihoroDir)
	if err != nil {
		return err
	}

	if opts.AllowLan {
		allowLan := true
		cfg.MihomoConfig.AllowLan = &allowLan
	}

	m := FromConfig(cfg, sf, mihoroDir)

	if !opts.Force && len(sf.Subscriptions) > 0 && sf.Active() != nil && m.binaryUsable() {
		fmt.Printf("%sAlready initialized.%s Use --force to reconfigure.\n", Green, Reset)
		return nil
	}

	if mirror == "" {
		mirror = cfg.GitHubMirror
	}

	alreadyLan := cfg.MihomoConfig.AllowLan != nil && *cfg.MihomoConfig.AllowLan
	if !opts.AllowLan && !alreadyLan {
		if promptBool("Allow LAN access?", "N") {
			allowLan := true
			cfg.MihomoConfig.AllowLan = &allowLan
		}
	}

	force := opts.Force
	arch := opts.Arch

	if _, err := m.EnsureSubscription(ctx, client); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if mirror != "" {
		fmt.Printf("Downloading components via %s...\n", mirror)
	} else {
		fmt.Println("Downloading components...")
	}
	binaryPlan, err := m.PrepareBinary(ctx, client, force, arch, mirror)
	if err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if _, err := m.EnsureGeodata(ctx, client, force, mirror); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if _, err := m.EnsureUI(ctx, client, force, mirror); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	var installTimers bool
	timerPath := "/etc/systemd/system/" + systemctl.UpdateTimerName
	timerExists := fileExists(timerPath)
	if timerExists && !opts.Force {
		fmt.Printf("  %sauto-update enabled%s (weekly, Mon 01:00)\n", Green, Reset)
		if cfg.GitHubMirror != "" {
			fmt.Printf("  mirror: %s\n", cfg.GitHubMirror)
		}
	} else {
		if promptBool("Enable component auto-update? (weekly, Mon 01:00)", "Y") {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			installTimers = true
			if mirrorFlag != "" {
				cfg.GitHubMirror = mirrorFlag
			} else {
				mirrorURL := promptString("Mirror URL (optional, leave empty to skip)", "")
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if mirrorURL != "" {
					cfg.GitHubMirror = mirrorURL
				}
			}
		}
	}

	if err := cfg.Save(ConfigPath(mihoroDir)); err != nil {
		fmt.Printf("  %swarning:%s save config: %v\n", Yellow, Reset, err)
	}

	if binaryPlan.ShouldInstall() {
		if _, err := m.InstallBinary(ctx, binaryPlan.TempFile); err != nil {
			return fmt.Errorf("install: %w", err)
		}
	}

	if _, err := m.EnsureService(ctx, force); err != nil {
		return fmt.Errorf("service: %w", err)
	}

	if _, err := m.EnsureServiceRunning(ctx, force); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	if installTimers {
		binPath, _ := os.Executable()
		if binPath == "" {
			binPath = "/usr/local/bin/mihoro"
		}
		if err := WriteTimerUnits(mihoroDir, binPath, cfg.GitHubMirror); err != nil {
			fmt.Printf("  %swarning:%s failed to install timers: %v\n", Yellow, Reset, err)
		} else {
			fmt.Printf("  %sauto-update enabled%s\n", Green, Reset)
		}
	}

	if cfg.UI != nil {
		printDashboardURLs(cfg)
	}
	if opts.AllowLan || (cfg.MihomoConfig.AllowLan != nil && *cfg.MihomoConfig.AllowLan) {
		printLanProxyInfo(cfg)
	}
	return nil
}

func (m *Mihoro) PrepareBinary(ctx context.Context, client *http.Client, force bool, archOverride string, mirror string) (BinaryPlan, error) {
	if !force && m.binaryUsable() {
		fmt.Printf("  mihomo core %sAlready present%s\n", Green, Reset)
		return BinaryPlan{SkipReason: "binary exists"}, nil
	}

	url, err := bin.ResolveBinaryURL(ctx, client, m.Config, archOverride)
	if err != nil {
		return BinaryPlan{}, fmt.Errorf("resolve binary URL: %w\n  (Hint: use --mirror <url> to download via a github mirror)", err)
	}

	tmpFile, err := os.CreateTemp("", "mihoro-binary-*")
	if err != nil {
		return BinaryPlan{}, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()

	if _, err := utils.Download(ctx, client, utils.DownloadOptions{
		URL:      url,
		DestPath: tmpPath,
		Label:    "  mihomo core",
		Retries:  utils.MaxRetries,
	}); err != nil {
		_ = os.Remove(tmpPath)
		return BinaryPlan{}, fmt.Errorf("download binary: %w\n  (Hint: use --mirror <url> to download via a github mirror)", err)
	}

	return BinaryPlan{TempFile: tmpPath}, nil
}

func (m *Mihoro) InstallBinary(ctx context.Context, tempFilePath string) (StageStatus, error) {
	_ = systemctl.Stop(systemctl.MihomoService)

	if err := utils.ExtractGzip(tempFilePath, m.BinaryPath, "  "); err != nil {
		return StageFailed, fmt.Errorf("extract binary: %w", err)
	}
	defer func() { _ = os.Remove(tempFilePath) }()

	if err := os.Chmod(m.BinaryPath, 0755); err != nil {
		return StageFailed, fmt.Errorf("chmod binary: %w", err)
	}
	return StageInstalled, nil
}

func (m *Mihoro) EnsureSubscription(ctx context.Context, client *http.Client) (StageStatus, error) {
	sub := m.Subs.Active()
	if sub == nil {
		return StageFailed, fmt.Errorf("no active subscription")
	}

	destPath := config.SubDownloadPath(m.ConfigDir, sub.Name)

	if _, err := os.Stat(destPath); err == nil {
		if err := config.CopyAfterOverride(destPath, m.MihomoCfg, &m.Config.MihomoConfig); err != nil {
			return StageFailed, fmt.Errorf("apply override: %w", err)
		}
		return StageSkipped, nil
	}

	ua := sub.UserAgent
	if ua == "" {
		ua = "clash/mihoro-go"
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return StageFailed, err
	}

	_, err := utils.Download(ctx, client, utils.DownloadOptions{
		URL:       sub.URL,
		DestPath:  destPath,
		UserAgent: ua,
		Headers:   sub.Headers,
		ProxyURL:  sub.Proxy,
		Retries:   2,
	})
	if err != nil {
		return StageFailed, err
	}

	if err := utils.TryDecodeBase64InPlace(destPath); err != nil {
		return StageFailed, fmt.Errorf("decode: %w", err)
	}

	if err := config.CopyAfterOverride(destPath, m.MihomoCfg, &m.Config.MihomoConfig); err != nil {
		return StageFailed, err
	}

	now := time.Now()
	sub.LastUpdate = now.Format(time.RFC3339)
	sub.LastStatus = "success"
	sub.LastError = ""
	if info, err := os.Stat(destPath); err == nil {
		sub.LastSize = info.Size()
	}
	m.Subs.Update(sub.Name, *sub)
	if err := m.Subs.Save(); err != nil {
		fmt.Printf("  warning: save subscription state: %v\n", err)
	}

	fmt.Println("  subscribe    Downloaded")
	return StageInstalled, nil
}

func (m *Mihoro) EnsureGeodata(ctx context.Context, client *http.Client, force bool, mirror string) (StageStatus, error) {
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
			if fileExists(geoipPath) && fileExists(geositePath) {
				fmt.Printf("  geodata      %sAlready present%s\n", Green, Reset)
				return StageSkipped, nil
			}
		}
		if force || !fileExists(geoipPath) {
			if _, err := utils.Download(ctx, client, utils.DownloadOptions{
				URL:      geox.Geoip,
				DestPath: geoipPath,
				Label:    "  geodata    ",
				Retries:  utils.MaxRetries,
			}); err != nil {
				return StageFailed, fmt.Errorf("download geoip.dat: %w", err)
			}
		}
		if force || !fileExists(geositePath) {
			if _, err := utils.Download(ctx, client, utils.DownloadOptions{
				URL:      geox.Geosite,
				DestPath: geositePath,
				Label:    "  geodata    ",
				Retries:  utils.MaxRetries,
			}); err != nil {
				return StageFailed, fmt.Errorf("download geosite.dat: %w", err)
			}
		}
	} else {
		mmdbPath := m.ConfigRoot + "/country.mmdb"
		if !force && fileExists(mmdbPath) {
			fmt.Printf("  geodata      %sAlready present%s\n", Green, Reset)
			return StageSkipped, nil
		}
		if _, err := utils.Download(ctx, client, utils.DownloadOptions{
			URL:      geox.Mmdb,
			DestPath: mmdbPath,
			Label:    "  geodata    ",
			Retries:  utils.MaxRetries,
			Timeout:  30 * time.Second,
		}); err != nil {
			return StageFailed, fmt.Errorf("download country.mmdb: %w", err)
		}
	}

	return StageInstalled, nil
}

func (m *Mihoro) EnsureUI(ctx context.Context, client *http.Client, force bool, mirror string) (StageStatus, error) {
	uiCfg := m.Config.UI
	if uiCfg == nil {
		return StageSkipped, nil
	}

	externalUI := m.Config.MihomoConfig.ExternalUI
	if externalUI == nil || *externalUI == "" {
		return StageSkipped, nil
	}

	targetDir := resolveExternalUIPath(m.ConfigRoot, *externalUI)

	if !force && fileExists(targetDir+"/index.html") {
		fmt.Printf("  web ui       %sAlready installed%s\n", Green, Reset)
		return StageSkipped, nil
	}

	if err := installUI(ctx, client, *uiCfg, targetDir); err != nil {
		return StageFailed, fmt.Errorf("install ui: %w", err)
	}

	return StageInstalled, nil
}

func (m *Mihoro) EnsureService(ctx context.Context, force bool) (StageStatus, error) {
	serviceContent := systemctl.RenderMihomoService(m.BinaryPath, m.ConfigRoot)
	servicePath := "/etc/systemd/system/" + systemctl.MihomoService

	if !force {
		if existing, err := os.ReadFile(servicePath); err == nil && string(existing) == serviceContent {
			fmt.Printf("  systemd      %sAlready configured%s\n", Green, Reset)
			return StageSkipped, nil
		}
	}

	if err := os.MkdirAll(filepath.Dir(servicePath), 0755); err != nil {
		return StageFailed, fmt.Errorf("create service dir: %w", err)
	}
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return StageFailed, fmt.Errorf("write service file: %w", err)
	}

	if err := systemctl.DaemonReload(); err != nil {
		return StageFailed, fmt.Errorf("daemon-reload: %w", err)
	}
	return StageInstalled, nil
}

func (m *Mihoro) EnsureServiceRunning(ctx context.Context, force bool) (StageStatus, error) {
	if !force && systemctl.IsActive(systemctl.MihomoService) && systemctl.IsEnabled(systemctl.MihomoService) {
		fmt.Printf("  start        %sAlready running%s\n", Green, Reset)
		return StageSkipped, nil
	}

	if force {
		_ = systemctl.Stop(systemctl.MihomoService)
	}

	if !systemctl.IsEnabled(systemctl.MihomoService) {
		if err := systemctl.Enable(systemctl.MihomoService); err != nil {
			return StageFailed, fmt.Errorf("enable: %w", err)
		}
	}
	if force || !systemctl.IsActive(systemctl.MihomoService) {
		if err := systemctl.Start(systemctl.MihomoService); err != nil {
			return StageFailed, fmt.Errorf("start: %w", err)
		}
	}
	return StageInstalled, nil
}

func (m *Mihoro) binaryUsable() bool {
	if _, err := os.Stat(m.BinaryPath); os.IsNotExist(err) {
		return false
	}
	_, err := m.InstalledVersion()
	return err == nil
}

func resolveExternalUIPath(configRoot, externalUI string) string {
	if strings.HasPrefix(externalUI, "/") {
		return externalUI
	}
	return configRoot + "/" + externalUI
}

func installUI(ctx context.Context, client *http.Client, uiCfg config.Ui, targetDir string) error {
	url := uiCfg.DownloadURL()
	tmpDir, err := os.MkdirTemp("", "mihoro-ui-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, "ui-archive")

	if _, err := utils.Download(ctx, client, utils.DownloadOptions{
		URL:      url,
		DestPath: archivePath,
		Label:    "  web ui     ",
		Retries:  utils.MaxRetries,
	}); err != nil {
		return err
	}

	extractDir := filepath.Join(tmpDir, "extracted")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return err
	}

	if strings.HasSuffix(url, ".tar.gz") || strings.HasSuffix(url, ".tgz") {
		cmd := exec.Command("tar", "-xzf", archivePath, "-C", extractDir)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("extract tar: %w\n%s", err, string(out))
		}
	} else {
		cmd := exec.Command("unzip", "-qo", archivePath, "-d", extractDir)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("extract zip: %w\n%s", err, string(out))
		}
	}

	entries, _ := os.ReadDir(extractDir)
	if len(entries) == 1 && entries[0].IsDir() {
		extractDir = filepath.Join(extractDir, entries[0].Name())
	}

	_ = os.RemoveAll(targetDir)
	if err := os.MkdirAll(filepath.Dir(targetDir), 0755); err != nil {
		return err
	}
	if err := os.Rename(extractDir, targetDir); err != nil {
		return fmt.Errorf("install ui to %s: %w", targetDir, err)
	}
	return nil
}

func printLanProxyInfo(cfg *config.Config) {
	ip := proxy.LocalIP()
	shell := proxy.DetectShell()
	port, socksPort := proxy.GetPorts(cfg.MihomoConfig)
	fmt.Println("LAN proxy enabled:")
	fmt.Printf("  %s\n", proxy.ExportCmd(shell, ip, port, socksPort))
	fmt.Println("  Use `mihoro proxy export-lan` to regenerate this command")
}
