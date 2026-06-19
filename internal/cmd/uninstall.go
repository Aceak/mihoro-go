package cmd

import (
	"mihoro-go/internal/mihoro"

	"github.com/spf13/cobra"
)

var uninstallYes bool

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall and remove mihoro and config",
	RunE: func(cmd *cobra.Command, args []string) error {
		m := mihoro.NewOrDefault(configPath)
		return m.Uninstall(CliCtx(), configPath, uninstallYes)
	},
}

func init() {
	uninstallCmd.Flags().BoolVarP(&uninstallYes, "yes", "y", false, "Skip prompts, remove everything")
}
