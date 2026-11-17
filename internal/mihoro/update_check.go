package mihoro

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"mihoro-go/internal/config"
	"mihoro-go/internal/upstream"
)

// geodataTarget describes one geodata file to download.
type geodataTarget struct {
	url       string
	dest      string
	assetName string // asset name in the meta-rules-dat release
}

// geodataTargets returns the geodata files to update. ok=false means geodata
// downloads are disabled in the config.
func (m *Mihoro) geodataTargets() (targets []geodataTarget, ok bool) {
	geox := m.Config.MihomoConfig.GeoxUrl
	if geox == nil {
		return nil, false
	}

	geodataMode := false
	if m.Config.MihomoConfig.GeodataMode != nil {
		geodataMode = *m.Config.MihomoConfig.GeodataMode
	}

	if geodataMode {
		return []geodataTarget{
			{url: geox.Geoip, dest: m.ConfigRoot + "/geoip.dat", assetName: "geoip.dat"},
			{url: geox.Geosite, dest: m.ConfigRoot + "/geosite.dat", assetName: "geosite.dat"},
		}, true
	}
	return []geodataTarget{
		{url: geox.Mmdb, dest: m.ConfigRoot + "/country.mmdb", assetName: "country.mmdb"},
	}, true
}

// uiReleaseRepo returns the GitHub repository that publishes the dashboard
// release. ok=false means this UI has no release tag to compare (the yacd-meta
// download comes from a branch, and custom UIs are user-provided).
func uiReleaseRepo(uiCfg config.Ui) (owner, repo string, ok bool) {
	switch uiCfg {
	case config.UiMetacubexd:
		return "MetaCubeX", "metacubexd", true
	case config.UiZashboard:
		return "Zephyruso", "zashboard", true
	default:
		return "", "", false
	}
}

// isMetaRulesDatURL reports whether raw points at the default geodata source
// (MetaCubeX/meta-rules-dat). Only that source has a known GitHub release, so
// only then is the API version check valid. Both direct GitHub URLs and URLs
// wrapped by a mirror (https://mirror/https://github.com/...) are recognized.
func isMetaRulesDatURL(raw string) bool {
	return strings.Contains(strings.ToLower(raw), "github.com/metacubex/meta-rules-dat/")
}

// geodataUpToDate compares the local geodata files with the SHA-256 digests
// published in the meta-rules-dat GitHub release.
//
// It returns (false, nil) when an update is needed or when the source is not
// the default repository. An error means the check itself failed; callers
// should warn and download anyway.
func (m *Mihoro) geodataUpToDate(ctx context.Context, client *http.Client,
	targets []geodataTarget) (bool, error) {
	for _, t := range targets {
		if !isMetaRulesDatURL(t.url) {
			return false, nil
		}
	}

	rel, err := upstream.LatestRelease(ctx, client, "MetaCubeX", "meta-rules-dat")
	if err != nil {
		return false, err
	}

	for _, t := range targets {
		asset := rel.AssetByName(t.assetName)
		if asset == nil || digestHex(asset.Digest) == "" {
			return false, nil
		}
		local, err := fileSHA256(t.dest)
		if err != nil {
			if os.IsNotExist(err) {
				return false, nil
			}
			return false, fmt.Errorf("hash %s: %w", t.dest, err)
		}
		if local != digestHex(asset.Digest) {
			return false, nil
		}
	}
	return true, nil
}

// uiTag matches a dashboard directory against the latest GitHub release.
// checked=false means no reliable tag exists for this UI. The returned tag is
// stored after a successful install.
func (m *Mihoro) uiUpToDate(ctx context.Context, client *http.Client, uiCfg config.Ui,
	targetDir string) (tag string, upToDate, checked bool, err error) {
	owner, repo, ok := uiReleaseRepo(uiCfg)
	if !ok {
		return "", false, false, nil
	}

	rel, err := upstream.LatestRelease(ctx, client, owner, repo)
	if err != nil {
		return "", false, true, err
	}

	st, err := loadUpdateState(m.ConfigDir)
	if err != nil {
		return rel.Tag, false, true, err
	}
	if st.UI[uiCfg.AsConfigValue()] != rel.Tag {
		return rel.Tag, false, true, nil
	}
	if _, err := os.Stat(targetDir); err != nil {
		return rel.Tag, false, true, nil
	}
	return rel.Tag, true, true, nil
}

// validateGzipFile checks that path starts with the gzip magic bytes.
func validateGzipFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	var head [2]byte
	if _, err := io.ReadFull(f, head[:]); err != nil {
		return fmt.Errorf("file too short")
	}
	if head[0] != 0x1f || head[1] != 0x8b {
		return fmt.Errorf("not a gzip file")
	}
	return nil
}

// validateZipFile checks that path starts with a zip local file header.
func validateZipFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	var head [4]byte
	if _, err := io.ReadFull(f, head[:]); err != nil {
		return fmt.Errorf("file too short")
	}
	if head[0] != 'P' || head[1] != 'K' {
		return fmt.Errorf("not a zip file")
	}
	return nil
}

// validateUIArchive returns a validator matching the archive type in url.
func validateUIArchive(url string) func(string) error {
	if strings.HasSuffix(url, ".tar.gz") || strings.HasSuffix(url, ".tgz") {
		return validateGzipFile
	}
	return validateZipFile
}

// validateGeodataFile rejects empty files and HTML error pages that a mirror
// may return with a 200 status.
func validateGeodataFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	var buf [512]byte
	n, err := f.Read(buf[:])
	if err != nil && n == 0 {
		return fmt.Errorf("empty file")
	}
	if n == 0 {
		return fmt.Errorf("empty file")
	}
	head := strings.ToLower(strings.TrimSpace(string(buf[:n])))
	for _, prefix := range []string{"<!doctype", "<html", "<?xml", "<head", "<body"} {
		if strings.HasPrefix(head, prefix) {
			return fmt.Errorf("received an HTML page instead of geodata")
		}
	}
	return nil
}
