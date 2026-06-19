package cmd

import (
	"mihoro-go/internal/cron"
	"mihoro-go/internal/mihoro"

	"github.com/spf13/cobra"
)

var cronCmd = &cobra.Command{
	Use:   "cron",
	Short: "Manage auto-update cron job",
}

var cronEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable auto-update cron job",
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := mihoro.New(configPath)
		if err != nil {
			return err
		}
		return cron.EnableAutoUpdate(m.Config.AutoUpdateInterval, m.Prefix)
	},
}

var cronDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable auto-update cron job",
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := mihoro.New(configPath)
		if err != nil {
			return err
		}
		return cron.DisableAutoUpdate(m.Prefix)
	},
}

var cronStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show auto-update cron job status",
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := mihoro.New(configPath)
		if err != nil {
			return err
		}
		return cron.Status(m.Prefix, m.ConfigPath)
	},
}

func init() {
	cronCmd.AddCommand(cronEnableCmd)
	cronCmd.AddCommand(cronDisableCmd)
	cronCmd.AddCommand(cronStatusCmd)
}
