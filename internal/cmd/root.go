package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"mihoro-go/internal/mihoro"
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

// ANSI color codes — defined in mihoro package.
var (
	colorGreen  = mihoro.Green
	colorYellow = mihoro.Yellow
	colorRed    = mihoro.Red
	colorReset  = mihoro.Reset
)

var rootCmd = &cobra.Command{
	Use:           "mihoro",
	Short:         "Mihomo CLI client on Linux",
	Long:          "mihoro manages the Mihomo proxy kernel — init, update, apply configs, and more.",
	Version:       versionString(),
	SilenceErrors: true,
	SilenceUsage:  true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		expandTilde(&configPath)
		ctx, cancel := context.WithCancel(context.Background())
		cliCtx, cliCancel = ctx, cancel
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)
		go func() {
			<-sigCh
			cancel()
			os.Exit(130)
		}()
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

func expandTilde(path *string) {
	if *path == "~" {
		*path = os.Getenv("HOME")
		return
	}
	if after, ok := strings.CutPrefix(*path, "~/"); ok {
		*path = filepath.Join(os.Getenv("HOME"), after)
	}
}

// CliCtx returns the signal-aware CLI context, valid for the duration of
// the current command execution.
func CliCtx() context.Context { return cliCtx }

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// ExitOnErr prints the error and exits appropriately.
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
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "~/.config/mihoro", "Path to mihoro config directory")

	rootCmd.AddCommand(initCmd)
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
	rootCmd.AddCommand(subCmd)
	rootCmd.AddCommand(upgradeCmd)
}
