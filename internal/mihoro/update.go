package mihoro

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"mihoro-go/internal/bin"
	"mihoro-go/internal/systemctl"
	"mihoro-go/internal/utils"
)

// PreparedUpdate is a fully downloaded and verified update. It is created by
// the Prepare* methods and only touches the live installation when Apply is
// called, so all components can be downloaded before anything is replaced.
type PreparedUpdate interface {
	Apply() error
	Discard()
}

// InstalledVersion returns the version string of the installed mihomo binary.
func (m *Mihoro) InstalledVersion() (string, error) {
	return binaryVersion(m.BinaryPath)
}

// binaryVersion runs path -v and extracts the version token.
func binaryVersion(path string) (string, error) {
	output, err := exec.Command(path, "-v").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("run %s -v: %w", path, err)
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

// --- core ---

// preparedCore holds a downloaded and verified mihomo binary.
type preparedCore struct {
	m       *Mihoro
	dir     string
	tmpPath string
}

// Apply stops the service, swaps the binary, and leaves the service stopped;
// the caller restarts it after every component has been applied.
func (u *preparedCore) Apply() error {
	if systemctl.IsActive(systemctl.MihomoService) {
		if err := systemctl.Stop(systemctl.MihomoService); err != nil {
			return fmt.Errorf("stop mihomo service: %w", err)
		}
	}
	if err := os.Rename(u.tmpPath, u.m.BinaryPath); err != nil {
		// The old binary is still in place; try to bring the service back.
		if startErr := systemctl.Start(systemctl.MihomoService); startErr != nil {
			return fmt.Errorf("install core: %w (service restart failed: %v)", err, startErr)
		}
		return fmt.Errorf("install core: %w", err)
	}
	return nil
}

// Discard removes the staging directory, including any temp binary.
func (u *preparedCore) Discard() {
	_ = os.RemoveAll(u.dir)
}

// PrepareCore resolves and downloads the latest core without touching the
// installed binary. A nil plan with a reason means "already up to date".
func (m *Mihoro) PrepareCore(ctx context.Context, client *http.Client,
	archOverride string) (plan PreparedUpdate, reason string, err error) {
	if _, statErr := os.Stat(m.BinaryPath); os.IsNotExist(statErr) {
		return nil, "", fmt.Errorf("mihomo binary not found at %s — run `mihoro init` first", m.BinaryPath)
	}

	resolved, err := bin.ResolveBinary(ctx, client, m.Config, archOverride)
	if err != nil {
		return nil, "", fmt.Errorf("resolve binary: %w", err)
	}

	if resolved.Version != "" {
		installed, verr := m.InstalledVersion()
		if verr == nil && installed == resolved.Version {
			// An explicit --arch asks for that build even when the version
			// number is unchanged.
			if archOverride == "" {
				return nil, fmt.Sprintf("already up to date (%s)", installed), nil
			}
			fmt.Printf("   Reinstalling mihomo core %s (arch %s)\n", installed, archOverride)
		} else if verr == nil && installed != "" {
			fmt.Printf("   Updating mihomo core: %s -> %s\n", installed, resolved.Version)
		}
	} else {
		fmt.Println("   Custom core URL: version check unavailable")
	}

	// Keep the staging directory next to the binary so the final rename stays
	// on the same filesystem.
	dir, err := os.MkdirTemp(filepath.Dir(m.BinaryPath), ".mihoro-core-*")
	if err != nil {
		return nil, "", fmt.Errorf("create temp dir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	gzPath := filepath.Join(dir, "mihomo.gz")
	if _, err := utils.Download(ctx, client, utils.DownloadOptions{
		URL:      resolved.URL,
		DestPath: gzPath,
		Retries:  utils.MaxRetries,
		Timeout:  5 * time.Minute,
		Validate: validateGzipFile,
	}); err != nil {
		cleanup()
		return nil, "", fmt.Errorf("download core: %w", err)
	}

	tmpBin := filepath.Join(dir, "mihomo")
	if err := utils.ExtractGzip(gzPath, tmpBin, "  "); err != nil {
		cleanup()
		return nil, "", fmt.Errorf("extract core: %w", err)
	}
	if err := os.Chmod(tmpBin, 0755); err != nil {
		cleanup()
		return nil, "", fmt.Errorf("chmod core: %w", err)
	}
	if _, err := binaryVersion(tmpBin); err != nil {
		cleanup()
		return nil, "", fmt.Errorf("verify downloaded core: %w", err)
	}

	return &preparedCore{m: m, dir: dir, tmpPath: tmpBin}, "", nil
}

// --- geodata ---

type geodataFile struct {
	tmpPath  string
	destPath string
}

type preparedGeodata struct {
	files []geodataFile
}

// Apply moves the downloaded geodata files into place.
func (u *preparedGeodata) Apply() error {
	for _, f := range u.files {
		if err := os.Rename(f.tmpPath, f.destPath); err != nil {
			return fmt.Errorf("install %s: %w", f.destPath, err)
		}
	}
	return nil
}

// Discard removes the downloaded temp files.
func (u *preparedGeodata) Discard() {
	for _, f := range u.files {
		_ = os.Remove(f.tmpPath)
	}
}

// PrepareGeodata downloads geodata into temp files. It returns a nil plan when
// the local files already match the latest release SHA-256.
func (m *Mihoro) PrepareGeodata(ctx context.Context, client *http.Client) (PreparedUpdate,
	string, error) {
	targets, ok := m.geodataTargets()
	if !ok {
		return nil, "geodata disabled", nil
	}

	upToDate, err := m.geodataUpToDate(ctx, client, targets)
	if err != nil {
		fmt.Printf("   %swarning:%s geodata version check failed: %v (downloading anyway)\n",
			Yellow, Reset, err)
	} else if upToDate {
		return nil, "already up to date (sha256 matches latest release)", nil
	}

	prepared := &preparedGeodata{}
	for _, t := range targets {
		if mkErr := os.MkdirAll(filepath.Dir(t.dest), 0755); mkErr != nil {
			prepared.Discard()
			return nil, "", fmt.Errorf("create geodata dir: %w", mkErr)
		}

		tmp, mkErr := os.CreateTemp(filepath.Dir(t.dest), "."+filepath.Base(t.dest)+".update-*")
		if mkErr != nil {
			prepared.Discard()
			return nil, "", fmt.Errorf("create temp file for %s: %w", t.dest, mkErr)
		}
		tmpPath := tmp.Name()
		_ = tmp.Close()

		if _, dlErr := utils.Download(ctx, client, utils.DownloadOptions{
			URL:      t.url,
			DestPath: tmpPath,
			Retries:  utils.MaxRetries,
			Timeout:  5 * time.Minute,
			Validate: validateGeodataFile,
		}); dlErr != nil {
			_ = os.Remove(tmpPath)
			prepared.Discard()
			return nil, "", fmt.Errorf("download %s: %w", filepath.Base(t.dest), dlErr)
		}
		if valErr := validateGeodataFile(tmpPath); valErr != nil {
			_ = os.Remove(tmpPath)
			prepared.Discard()
			return nil, "", fmt.Errorf("%s: %w", filepath.Base(t.dest), valErr)
		}
		if chErr := os.Chmod(tmpPath, 0644); chErr != nil {
			_ = os.Remove(tmpPath)
			prepared.Discard()
			return nil, "", fmt.Errorf("chmod %s: %w", tmpPath, chErr)
		}

		prepared.files = append(prepared.files, geodataFile{tmpPath: tmpPath, destPath: t.dest})
	}
	return prepared, "", nil
}

// --- ui ---

// preparedUI holds an extracted dashboard directory ready to be swapped in.
type preparedUI struct {
	m         *Mihoro
	baseDir   string
	srcDir    string
	targetDir string
	uiKey     string
	tag       string // latest release tag, empty when unknown
}

// Apply swaps the new directory into place, keeping the old one until the new
// one has been moved successfully.
func (u *preparedUI) Apply() error {
	parent := filepath.Dir(u.targetDir)

	backup := ""
	if _, err := os.Stat(u.targetDir); err == nil {
		backup, err = os.MkdirTemp(parent, ".mihoro-ui-old-*")
		if err != nil {
			return fmt.Errorf("create ui backup dir: %w", err)
		}
		_ = os.Remove(backup) // rename needs a non-existing destination
		if err := os.Rename(u.targetDir, backup); err != nil {
			return fmt.Errorf("back up existing ui: %w", err)
		}
	}

	if err := os.Rename(u.srcDir, u.targetDir); err != nil {
		if backup != "" {
			_ = os.Rename(backup, u.targetDir)
		}
		return fmt.Errorf("install ui to %s: %w", u.targetDir, err)
	}
	if backup != "" {
		if err := os.RemoveAll(backup); err != nil {
			fmt.Printf("   %swarning:%s remove old ui backup %s: %v\n", Yellow, Reset, backup, err)
		}
	}

	if u.tag != "" {
		st, err := loadUpdateState(u.m.ConfigDir)
		if err != nil {
			// A corrupt state file must not block updating it.
			fmt.Printf("   %swarning:%s %v (resetting update state)\n", Yellow, Reset, err)
			st = newUpdateState()
		}
		st.UI[u.uiKey] = u.tag
		if err := saveUpdateState(u.m.ConfigDir, st); err != nil {
			fmt.Printf("   %swarning:%s %v\n", Yellow, Reset, err)
		}
	}
	return nil
}

// Discard removes the staging directory.
func (u *preparedUI) Discard() {
	_ = os.RemoveAll(u.baseDir)
}

// PrepareUI downloads and extracts the dashboard into a staging directory.
// It returns a nil plan when the installed release already matches the latest
// GitHub tag.
func (m *Mihoro) PrepareUI(ctx context.Context, client *http.Client) (PreparedUpdate,
	string, error) {
	uiCfg := m.Config.UI
	if uiCfg == nil {
		return nil, "ui disabled", nil
	}

	externalUI := m.Config.MihomoConfig.ExternalUI
	if externalUI == nil || *externalUI == "" {
		return nil, "external-ui not configured", nil
	}

	targetDir := resolveExternalUIPath(m.ConfigRoot, *externalUI)

	tag, upToDate, _, err := m.uiUpToDate(ctx, client, *uiCfg, targetDir)
	if err != nil {
		fmt.Printf("   %swarning:%s ui version check failed: %v (downloading anyway)\n",
			Yellow, Reset, err)
	} else if upToDate {
		return nil, fmt.Sprintf("already up to date (%s)", tag), nil
	}

	parent := filepath.Dir(targetDir)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return nil, "", fmt.Errorf("create ui parent dir: %w", err)
	}
	baseDir, err := os.MkdirTemp(parent, ".mihoro-ui-*")
	if err != nil {
		return nil, "", fmt.Errorf("create ui staging dir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(baseDir) }

	url := uiCfg.DownloadURL()
	archivePath := filepath.Join(baseDir, "ui-archive")
	if _, err := utils.Download(ctx, client, utils.DownloadOptions{
		URL:      url,
		DestPath: archivePath,
		Label:    "  web ui     ",
		Retries:  utils.MaxRetries,
		Timeout:  5 * time.Minute,
		Validate: validateUIArchive(url),
	}); err != nil {
		cleanup()
		return nil, "", err
	}

	extractDir := filepath.Join(baseDir, "extracted")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		cleanup()
		return nil, "", err
	}
	if err := extractUIArchive(archivePath, extractDir, url); err != nil {
		cleanup()
		return nil, "", err
	}

	if entries, err := os.ReadDir(extractDir); err == nil && len(entries) == 1 && entries[0].IsDir() {
		extractDir = filepath.Join(extractDir, entries[0].Name())
	}

	return &preparedUI{
		m:         m,
		baseDir:   baseDir,
		srcDir:    extractDir,
		targetDir: targetDir,
		uiKey:     uiCfg.AsConfigValue(),
		tag:       tag,
	}, "", nil
}

// extractUIArchive extracts a dashboard archive into dst.
func extractUIArchive(archivePath, dst, url string) error {
	if strings.HasSuffix(url, ".tar.gz") || strings.HasSuffix(url, ".tgz") {
		cmd := exec.Command("tar", "-xzf", archivePath, "-C", dst)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("extract tar: %w\n%s", err, string(out))
		}
		return nil
	}
	cmd := exec.Command("unzip", "-qo", archivePath, "-d", dst)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("extract zip: %w\n%s", err, string(out))
	}
	return nil
}

// RestartService restarts the mihomo service.
func (m *Mihoro) RestartService() error {
	return systemctl.Restart(systemctl.MihomoService)
}
