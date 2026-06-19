package cmd

import (
	"fmt"

	"mihoro-go/internal/upgrade"
	ver "mihoro-go/internal/version"

	"github.com/spf13/cobra"
)

var (
	upgradeYes   bool
	upgradeCheck bool
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade mihoro to the latest version",
	RunE:  runUpgrade,
}

func init() {
	upgradeCmd.Flags().BoolVarP(&upgradeYes, "yes", "y", false, "Skip confirmation prompt")
	upgradeCmd.Flags().BoolVar(&upgradeCheck, "check", false, "Only check for updates, don't install")
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	ctx := CliCtx()
	client := newHTTPClient()

	if upgradeCheck {
		latest, err := upgrade.CheckForUpdate(ctx, client)
		if err != nil {
			return err
		}
		if latest != "" {
			fmt.Printf("mihoro: New version available: %s\n", latest)
			fmt.Println("  -> Run `mihoro upgrade` to update")
		} else {
			fmt.Printf("mihoro: You're running the latest version (%s)\n", ver.Version)
		}
		return nil
	}

	return upgrade.RunUpgrade(ctx, client)
}
