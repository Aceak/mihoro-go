package mihoro

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"mihoro-go/internal/bin"
	"mihoro-go/internal/systemctl"
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

func (m *Mihoro) UpdateCore(ctx context.Context, client *http.Client, archOverride, mirror string) (StageStatus, error) {
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

	if _, err := utils.Download(ctx, client, utils.DownloadOptions{
		URL:      resolved.URL,
		DestPath: tmpPath,
		Retries:  utils.MaxRetries,
	}); err != nil {
		return StageFailed, fmt.Errorf("download core: %w", err)
	}

	_ = systemctl.Stop(systemctl.MihomoService)

	if err := utils.ExtractGzip(tmpPath, m.BinaryPath, "  "); err != nil {
		return StageFailed, fmt.Errorf("extract core: %w", err)
	}

	if err := os.Chmod(m.BinaryPath, 0755); err != nil {
		return StageFailed, fmt.Errorf("chmod core: %w", err)
	}

	return StageInstalled, nil
}

func (m *Mihoro) UpdateGeodata(ctx context.Context, client *http.Client, mirror string) (StageStatus, error) {

	geox := m.Config.MihomoConfig.GeoxUrl
	if geox == nil {
		return StageSkipped, nil
	}

	geodataMode := false
	if m.Config.MihomoConfig.GeodataMode != nil {
		geodataMode = *m.Config.MihomoConfig.GeodataMode
	}

	if geodataMode {
		if _, err := utils.Download(ctx, client, utils.DownloadOptions{
			URL:      geox.Geoip,
			DestPath: m.ConfigRoot + "/geoip.dat",
			Retries:  utils.MaxRetries,
			Timeout:   30 * time.Second,
		}); err != nil {
			return StageFailed, fmt.Errorf("download geoip.dat: %w", err)
		}
		if _, err := utils.Download(ctx, client, utils.DownloadOptions{
			URL:      geox.Geosite,
			DestPath: m.ConfigRoot + "/geosite.dat",
			Retries:  utils.MaxRetries,
			Timeout:   30 * time.Second,
		}); err != nil {
			return StageFailed, fmt.Errorf("download geosite.dat: %w", err)
		}
	} else {
		if _, err := utils.Download(ctx, client, utils.DownloadOptions{
			URL:      geox.Mmdb,
			DestPath: m.ConfigRoot + "/country.mmdb",
			Retries:  utils.MaxRetries,
			Timeout:   30 * time.Second,
		}); err != nil {
			return StageFailed, fmt.Errorf("download country.mmdb: %w", err)
		}
	}

	return StageInstalled, nil
}

func (m *Mihoro) UpdateUI(ctx context.Context, client *http.Client, mirror string) (StageStatus, error) {

	uiCfg := m.Config.UI
	if uiCfg == nil {
		return StageSkipped, nil
	}

	externalUI := m.Config.MihomoConfig.ExternalUI
	if externalUI == nil || *externalUI == "" {
		return StageSkipped, nil
	}

	targetDir := resolveExternalUIPath(m.ConfigRoot, *externalUI)

	if err := installUI(ctx, client, *uiCfg, targetDir); err != nil {
		return StageFailed, fmt.Errorf("install ui: %w", err)
	}

	return StageInstalled, nil
}

func (m *Mihoro) RestartService() error {
	return systemctl.Restart(systemctl.MihomoService)
}
