package mihoro

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"mihoro-go/internal/bin"
	"mihoro-go/internal/config"
	"mihoro-go/internal/systemctl"
	"mihoro-go/internal/ui"
	"mihoro-go/internal/utils"
)

// InstalledVersion returns the version string of the installed mihomo binary.
func (m *Mihoro) InstalledVersion() (string, error) {
	output, err := exec.Command(m.BinaryPath, "-v").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("run %s -v: %w", m.BinaryPath, err)
	}
	return extractMihomoVersion(string(output)), nil
}

func extractMihomoVersion(output string) string {
	for _, token := range strings.Fields(output) {
		if v := normalizeVersionToken(token); v != "" {
			return v
		}
	}
	return ""
}

func normalizeVersionToken(token string) string {
	token = strings.TrimRight(token, ",;:()[]")
	if rest, ok := strings.CutPrefix(token, "v"); ok {
		if len(rest) > 0 && rest[0] >= '0' && rest[0] <= '9' {
			return token
		}
	}
	if strings.HasPrefix(token, "alpha-") {
		return token
	}
	if len(token) > 0 && token[0] >= '0' && token[0] <= '9' && strings.Contains(token, ".") {
		return "v" + token
	}
	return ""
}

func (m *Mihoro) UpdateCore(ctx context.Context, client *http.Client, archOverride string) (StageStatus, error) {
	if _, err := os.Stat(m.BinaryPath); os.IsNotExist(err) {
		return StageFailed, fmt.Errorf("mihomo binary not found at %s — run `mihoro init` first", m.BinaryPath)
	}

	resolved, err := bin.ResolveBinary(ctx, client, m.Config, archOverride)
	if err != nil {
		return StageFailed, fmt.Errorf("resolve binary: %w", err)
	}

	if resolved.Version != "" {
		installed, err := m.InstalledVersion()
		if err == nil && installed == resolved.Version {
			fmt.Printf("   Mihomo core is already up to date (%s)\n", installed)
			return StageSkipped, nil
		} else if err == nil && installed != "" {
			fmt.Printf("   Updating mihomo core: %s -> %s\n", installed, resolved.Version)
		}
	}

	tmpFile, err := os.CreateTemp("", "mihoro-update-core-*")
	if err != nil {
		return StageFailed, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := utils.DownloadFile(ctx, client, resolved.URL, tmpPath, m.Config.MihoroUserAgent, ""); err != nil {
		return StageFailed, fmt.Errorf("download core: %w", err)
	}

	fmt.Println("   Stopping mihomo.service before overwriting...")
	sctl := systemctl.New(m.SystemdScope)
	_ = sctl.Stop("mihomo.service")

	if err := utils.ExtractGzip(tmpPath, m.BinaryPath, "   "); err != nil {
		return StageFailed, fmt.Errorf("extract core: %w", err)
	}

	if err := os.Chmod(m.BinaryPath, 0755); err != nil {
		return StageFailed, fmt.Errorf("chmod core: %w", err)
	}

	return StageInstalled, nil
}

func (m *Mihoro) UpdateConfig(ctx context.Context, client *http.Client) (StageStatus, error) {
	if err := utils.DownloadFile(ctx, client, m.Config.RemoteConfigURL, m.ConfigPath, m.Config.MihoroUserAgent, ""); err != nil {
		return StageFailed, fmt.Errorf("download config: %w", err)
	}

	if err := utils.TryDecodeBase64InPlace(m.ConfigPath); err != nil {
		return StageFailed, fmt.Errorf("decode config: %w", err)
	}

	if _, err := config.ApplyOverride(m.ConfigPath, &m.Config.MihomoConfig); err != nil {
		return StageFailed, fmt.Errorf("apply override: %w", err)
	}

	fmt.Println("   Updated and applied config overrides")
	return StageInstalled, nil
}

func (m *Mihoro) UpdateGeodata(ctx context.Context, client *http.Client) (StageStatus, error) {
	geox := m.Config.MihomoConfig.GeoxUrl
	if geox == nil {
		return StageSkipped, nil
	}

	geodataMode := false
	if m.Config.MihomoConfig.GeodataMode != nil {
		geodataMode = *m.Config.MihomoConfig.GeodataMode
	}

	if geodataMode {
		if err := utils.DownloadFile(ctx, client, geox.Geoip, m.ConfigRoot+"/geoip.dat", m.Config.MihoroUserAgent, ""); err != nil {
			return StageFailed, fmt.Errorf("download geoip.dat: %w", err)
		}
		if err := utils.DownloadFile(ctx, client, geox.Geosite, m.ConfigRoot+"/geosite.dat", m.Config.MihoroUserAgent, ""); err != nil {
			return StageFailed, fmt.Errorf("download geosite.dat: %w", err)
		}
	} else {
		if err := utils.DownloadFile(ctx, client, geox.Mmdb, m.ConfigRoot+"/country.mmdb", m.Config.MihoroUserAgent, ""); err != nil {
			return StageFailed, fmt.Errorf("download country.mmdb: %w", err)
		}
	}

	fmt.Println("   Downloaded and updated geodata")
	return StageInstalled, nil
}

func (m *Mihoro) UpdateUI(ctx context.Context, client *http.Client) (StageStatus, error) {
	uiCfg := m.Config.UI
	if uiCfg == nil {
		return StageSkipped, nil
	}

	externalUI := m.Config.MihomoConfig.ExternalUI
	if externalUI == nil || *externalUI == "" {
		return StageSkipped, nil
	}

	targetDir := resolveExternalUIPath(m.ConfigRoot, *externalUI)

	if err := ui.InstallUI(ctx, client, *uiCfg, targetDir, m.Config.MihoroUserAgent, "   "); err != nil {
		return StageFailed, fmt.Errorf("install ui: %w", err)
	}

	return StageInstalled, nil
}

func (m *Mihoro) RestartService() error {
	sctl := systemctl.New(m.SystemdScope)
	return sctl.Restart("mihomo.service")
}
