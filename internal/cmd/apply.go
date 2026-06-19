package cmd

import (
	"mihoro-go/internal/mihoro"

	"github.com/spf13/cobra"
)

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply mihomo config overrides and restart mihomo.service",
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := mihoro.New(configPath)
		if err != nil {
			return err
		}
		return m.Apply()
	},
}
