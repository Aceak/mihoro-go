package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"mihoro-go/internal/upgrade"
	ver "mihoro-go/internal/version"

	"github.com/spf13/cobra"
)

var (
	upgradeYes    bool
	upgradeCheck  bool
	upgradeMirror string
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
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	ctx := CliCtx()

	if upgradeMirror != "" {
		if !strings.Contains(upgradeMirror, "://") || !strings.Contains(upgradeMirror[strings.Index(upgradeMirror, "://")+3:], ".") {
			return fmt.Errorf("--mirror requires a valid URL (e.g. --mirror \"https://ghfast.top\")")
		}
		_ = os.Setenv("MIHORO_GITHUB_MIRROR", upgradeMirror)
	}

	client := newHTTPClient()

	if upgradeCheck {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		latest, err := upgrade.CheckForUpdate(ctx, client)
		if err != nil {
			return hintMirror(err, upgradeMirror)
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
	return hintMirror(upgrade.RunUpgrade(ctx, client), upgradeMirror)
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
