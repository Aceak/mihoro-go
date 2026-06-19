package cmd

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"mihoro-go/internal/bin"
	"mihoro-go/internal/mihoro"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize mihoro: download binary, config, geodata, and set up systemd service",
	RunE:  runInit,
}

var (
	initForce     bool
	initYes       bool
	initArch      string
	initSystem    bool
	initMirror    string
	initUserAgent string
	initSubscribe string
	initAllowLan  bool
)

func init() {
	initCmd.Flags().BoolVar(&initForce, "force", false, "Re-download all artifacts even if they already exist")
	initCmd.Flags().BoolVarP(&initYes, "yes", "y", false, "Non-interactive mode: fail if required config fields are missing")
	initCmd.Flags().StringVar(&initArch, "arch", "", "Override architecture detection")
	initCmd.Flags().BoolVar(&initSystem, "system", false, "Install as system-level service (requires root)")
	initCmd.Flags().StringVar(&initMirror, "mirror", "", "GitHub mirror base URL (e.g. https://ghfast.top)")
	initCmd.Flags().StringVar(&initUserAgent, "ua", "", "Override User-Agent header")
	initCmd.Flags().StringVarP(&initSubscribe, "subscribe", "s", "", "Remote subscription URL")
	initCmd.Flags().BoolVar(&initAllowLan, "allow-lan", false, "Allow LAN connections to the proxy")
}

var setupCmd = &cobra.Command{
	Use:    "setup",
	Short:  "Deprecated: use `mihoro init` instead",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		initYes = true
		return runInit(cmd, args)
	},
}

func runInit(cmd *cobra.Command, args []string) error {
	if initMirror != "" {
		if !strings.Contains(initMirror, "://") || !strings.Contains(initMirror[strings.Index(initMirror, "://")+3:], ".") {
			return fmt.Errorf("--mirror requires a valid URL (e.g. --mirror \"https://ghfast.top\")")
		}
		_ = os.Setenv("MIHORO_GITHUB_MIRROR", initMirror)
	}
	if initArch != "" {
		if _, err := bin.ValidateArch(initArch); err != nil {
			return fmt.Errorf("--arch: %w", err)
		}
	}
	if initUserAgent != "" && strings.TrimSpace(initUserAgent) == "" {
		return fmt.Errorf("--ua cannot be empty")
	}

	client := newHTTPClient()

	opts := mihoro.InitOptions{
		Force:     initForce,
		Yes:       initYes,
		Arch:      initArch,
		System:    initSystem,
		UserAgent: initUserAgent,
		Subscribe: initSubscribe,
		AllowLan:  initAllowLan,
	}

	return mihoro.RunInit(CliCtx(), client, configPath, opts)
}

func newHTTPClient() *http.Client {
	return &http.Client{}
}
