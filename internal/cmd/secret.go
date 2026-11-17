package cmd

import (
	"mihoro-go/internal/mihoro"

	"github.com/spf13/cobra"
)

var secretResetDisable bool

var secretCmd = &cobra.Command{
	Use:   "secret",
	Short: "Manage the mihomo API/dashboard secret",
	Long:  "Manage the secret used by the mihomo external controller and dashboard.",
}

var secretResetCmd = &cobra.Command{
	Use:     "reset",
	Aliases: []string{"rotate"},
	Short:   "Set a new API secret (empty input generates one), apply it, and restart mihomo",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return mihoro.RunSecretReset(configPath, secretResetDisable)
	},
}

func init() {
	secretCmd.AddCommand(secretResetCmd)
	secretResetCmd.Flags().BoolVar(&secretResetDisable, "disable", false,
		"Remove the API secret and disable authentication")
}
