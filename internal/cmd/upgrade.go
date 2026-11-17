package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"mihoro-go/internal/config"
	"mihoro-go/internal/mihoro"
	"mihoro-go/internal/upgrade"
	"mihoro-go/internal/utils"
	ver "mihoro-go/internal/version"

	"github.com/spf13/cobra"
)

var (
	upgradeYes    bool
	upgradeCheck  bool
	upgradeMirror string
	upgradeProxy  string
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade mihoro to the latest version",
	RunE:  runUpgrade,
}

func init() {
	upgradeCmd.Flags().BoolVarP(&upgradeYes, "yes", "y", false, "Skip confirmation prompt")
	upgradeCmd.Flags().BoolVar(&upgradeCheck, "check", false, "Only check for updates, don't install")
	upgradeCmd.Flags().StringVar(&upgradeMirror, "mirror", "", "GitHub mirror base URL (e.g. https://ghfast.top)")
	upgradeCmd.Flags().StringVar(&upgradeProxy, "proxy", "",
		"HTTP/SOCKS proxy for downloads (e.g. http://127.0.0.1:7890)")
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	ctx := CliCtx()

	// config.toml supplies persistent mirror and proxy defaults.
	cfg, cfgErr := config.Load(mihoro.ConfigPath(configPath))
	if cfgErr != nil {
		// A missing config returns nil, nil; a real error means the file is
		// unreadable or malformed. Do not block upgrade on it, but surface
		// the problem instead of silently ignoring it.
		fmt.Printf("warning: load config: %v (saved mirror and proxy ignored)\n", cfgErr)
	}

	// Mirror: the flag wins, config.toml is the persistent fallback.
	rawMirror := upgradeMirror
	if rawMirror == "" && cfg != nil {
		rawMirror = cfg.GitHubMirror
	}
	mirror, err := utils.SetMirrorEnv(rawMirror)
	if err != nil {
		return fmt.Errorf("--mirror: %w", err)
	}

	// Proxy: explicit only. The flag wins over config.toml.
	proxy, err := utils.NormalizeProxy(upgradeProxy)
	if err != nil {
		return fmt.Errorf("--proxy: %w", err)
	}
	if proxy == "" && cfg != nil {
		proxy, err = utils.NormalizeProxy(cfg.Proxy)
		if err != nil {
			return fmt.Errorf("config proxy: %w", err)
		}
	}
	if proxy != "" {
		fmt.Printf("Using proxy %s\n", utils.RedactProxy(proxy))
	}

	client := newHTTPClient(proxy)

	if upgradeCheck {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		latest, err := upgrade.CheckForUpdate(ctx, client)
		if err != nil {
			return hintMirror(err, mirror)
		}
		if latest != "" {
			fmt.Printf("mihoro: New version available: %s\n", latest)
			fmt.Println("  -> Run `mihoro upgrade` to update")
		} else {
			fmt.Printf("mihoro: You're running the latest version (%s)\n", ver.Version)
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	return hintMirror(upgrade.RunUpgrade(ctx, client), mirror)
}

func hintMirror(err error, mirror string) error {
	if err == nil || mirror != "" {
		return err
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return err
	}
	if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline") {
		return fmt.Errorf("%w\n  Tip: use --mirror if GitHub is slow in your region", err)
	}
	return err
}
