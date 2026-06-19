package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"mihoro-go/internal/version"

	"github.com/spf13/cobra"
)

var (
	configPath string

	// cliCtx is a signal-aware context created before every command and
	// cancelled after it finishes. Command RunE handlers should use this
	// instead of creating their own signal.NotifyContext.
	cliCtx    context.Context
	cliCancel context.CancelFunc
)

// ANSI color codes for terminal output.
const (
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorReset  = "\033[0m"
)

var rootCmd = &cobra.Command{
	Use:           "mihoro",
	Short:         "Mihomo CLI client on Linux",
	Long:          "mihoro manages the Mihomo proxy kernel — init, update, apply configs, and more.",
	Version:       versionString(),
	SilenceErrors: true,
	SilenceUsage:  true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		cliCtx, cliCancel = signal.NotifyContext(context.Background(), os.Interrupt)
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if cliCancel != nil {
			cliCancel()
		}
		return nil
	},
}

func versionString() string {
	v := version.Version
	if version.Commit != "" {
		v += " (" + version.Commit + ")"
	}
	return v
}

// CliCtx returns the signal-aware CLI context, valid for the duration of
// the current command execution.
func CliCtx() context.Context { return cliCtx }

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// ExitOnErr prints the error and exits appropriately.
// Context cancellation gets a clean message and exit code 130.
func ExitOnErr(err error) {
	if err == nil {
		return
	}
	if errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "\n%s cancelled%s\n", colorYellow, colorReset)
		os.Exit(130)
	}
	fmt.Fprintf(os.Stderr, "%serror:%s %v\n", colorRed, colorReset, err)
	os.Exit(1)
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "~/.config/mihoro.toml", "Path to mihoro config file")

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(applyCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(logCmd)
	rootCmd.AddCommand(proxyCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(completionCmd)
	rootCmd.AddCommand(cronCmd)
	rootCmd.AddCommand(upgradeCmd)
}
