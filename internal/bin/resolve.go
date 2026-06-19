package bin

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"strings"

	"mihoro-go/internal/config"
	"mihoro-go/internal/utils"
)

const (
	stableVersionURL = "https://github.com/MetaCubeX/mihomo/releases/latest/download/version.txt"
	alphaVersionURL  = "https://github.com/MetaCubeX/mihomo/releases/download/Prerelease-Alpha/version.txt"
)

// ResolvedBinary holds the download URL and optional version of a resolved mihomo binary.
type ResolvedBinary struct {
	URL     string
	Version string // empty when using a configured URL
}

// supportedArchs is the complete list of mihomo architectures.
var supportedArchs = []string{
	"386", "386-go120", "386-go123", "386-softfloat",
	"amd64", "amd64-compatible", "amd64-v1", "amd64-v1-go120", "amd64-v1-go123",
	"amd64-v2", "amd64-v2-go120", "amd64-v2-go123",
	"amd64-v3", "amd64-v3-go120", "amd64-v3-go123",
	"arm64", "armv5", "armv6", "armv7",
	"loong64-abi1", "loong64-abi2",
	"mips-hardfloat", "mips-softfloat", "mips64", "mips64le",
	"mipsle-hardfloat", "mipsle-softfloat",
	"ppc64le", "riscv64", "s390x",
}

// DetectArch maps runtime.GOARCH to mihomo's arch naming convention.
// Defaults: amd64→amd64-compatible (max compatibility), arm→armv7, loong64→loong64-abi2.
func DetectArch() (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		return "amd64-compatible", nil
	case "arm64":
		return "arm64", nil
	case "arm":
		return "armv7", nil
	case "386":
		return "386", nil
	case "mips64":
		return "mips64", nil
	case "mips64le":
		return "mips64le", nil
	case "mips":
		return "mips-softfloat", nil
	case "mipsle":
		return "mipsle-softfloat", nil
	case "ppc64le":
		return "ppc64le", nil
	case "riscv64":
		return "riscv64", nil
	case "s390x":
		return "s390x", nil
	case "loong64":
		return "loong64-abi2", nil
	default:
		return "", fmt.Errorf("unsupported architecture: %s (use --arch to specify manually)", runtime.GOARCH)
	}
}

// ValidateArch checks that arch is in the supported list. Returns the arch if valid,
// or an error with suggestions.
func ValidateArch(arch string) (string, error) {
	for _, a := range supportedArchs {
		if a == arch {
			return arch, nil
		}
	}

	// Find suggestions with matching prefix
	var suggestions []string
	prefix := arch
	if len(prefix) > 3 {
		prefix = prefix[:3]
	}
	for _, a := range supportedArchs {
		if strings.HasPrefix(a, prefix) {
			suggestions = append(suggestions, a)
		}
	}

	if len(suggestions) > 0 {
		return "", fmt.Errorf("unsupported architecture: %q\nDid you mean: %s", arch, strings.Join(suggestions, ", "))
	}
	return "", fmt.Errorf("unsupported architecture: %q\nSupported: %s", arch, strings.Join(supportedArchs, ", "))
}

// BuildDownloadURL constructs the download URL for a given version, arch, and channel.
func BuildDownloadURL(version, arch string, channel config.MihomoChannel) string {
	switch channel {
	case config.ChannelStable:
		return fmt.Sprintf("https://github.com/MetaCubeX/mihomo/releases/latest/download/mihomo-linux-%s-%s.gz", arch, version)
	case config.ChannelAlpha:
		return fmt.Sprintf("https://github.com/MetaCubeX/mihomo/releases/download/Prerelease-Alpha/mihomo-linux-%s-%s.gz", arch, version)
	default:
		return fmt.Sprintf("https://github.com/MetaCubeX/mihomo/releases/latest/download/mihomo-linux-%s-%s.gz", arch, version)
	}
}

// FetchLatestVersion fetches the latest mihomo version for the given channel from GitHub.
func FetchLatestVersion(ctx context.Context, client *http.Client, channel config.MihomoChannel, userAgent string) (string, error) {
	url := stableVersionURL
	if channel == config.ChannelAlpha {
		url = alphaVersionURL
	}

	resolvedURL := utils.ResolveDownloadURL(url)
	req, err := http.NewRequestWithContext(ctx, "GET", resolvedURL, nil)
	if err != nil {
		return "", fmt.Errorf("create version request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch version from %s: %w", resolvedURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("fetch version: HTTP %d", resp.StatusCode)
	}

	// Read body — version.txt is small (< 100 bytes)
	var buf [128]byte
	n, err := resp.Body.Read(buf[:])
	if err != nil && err.Error() != "EOF" {
		return "", fmt.Errorf("read version response: %w", err)
	}
	version := strings.TrimSpace(string(buf[:n]))
	if version == "" {
		return "", fmt.Errorf("received empty version from GitHub")
	}
	return version, nil
}

// ResolveBinary resolves the mihomo binary download URL.
//
// If the config has remote_mihomo_binary_url set, returns it directly.
// Otherwise auto-detects architecture and fetches the latest version from GitHub.
func ResolveBinary(ctx context.Context, client *http.Client, cfg *config.Config, archOverride string) (*ResolvedBinary, error) {
	// If a URL is explicitly configured, use it directly
	if cfg.RemoteMihomoBinaryURL != nil && *cfg.RemoteMihomoBinaryURL != "" {
		return &ResolvedBinary{
			URL:     *cfg.RemoteMihomoBinaryURL,
			Version: "",
		}, nil
	}

	// Determine architecture: CLI override > config > auto-detect
	arch := ""
	if archOverride != "" {
		var err error
		arch, err = ValidateArch(archOverride)
		if err != nil {
			return nil, err
		}
	} else if cfg.MihomoArch != nil && *cfg.MihomoArch != "" {
		var err error
		arch, err = ValidateArch(*cfg.MihomoArch)
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		arch, err = DetectArch()
		if err != nil {
			return nil, err
		}
	}

	version, err := FetchLatestVersion(ctx, client, cfg.MihomoChannel, cfg.MihoroUserAgent)
	if err != nil {
		return nil, fmt.Errorf("fetch latest version: %w", err)
	}

	url := BuildDownloadURL(version, arch, cfg.MihomoChannel)
	return &ResolvedBinary{URL: url, Version: version}, nil
}

// ResolveBinaryURL is a convenience wrapper that returns just the download URL.
func ResolveBinaryURL(ctx context.Context, client *http.Client, cfg *config.Config, archOverride string) (string, error) {
	rb, err := ResolveBinary(ctx, client, cfg, archOverride)
	if err != nil {
		return "", err
	}
	return rb.URL, nil
}
