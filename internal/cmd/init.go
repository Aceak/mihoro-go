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
	initForce    bool
	initArch     string
	initMirror   string
	initAllowLan bool
)

func init() {
	initCmd.Flags().BoolVar(&initForce, "force", false, "Re-download all artifacts even if they already exist")
	initCmd.Flags().StringVar(&initArch, "arch", "", "Override architecture detection")
	initCmd.Flags().StringVar(&initMirror, "mirror", "", "GitHub mirror base URL (e.g. https://ghfast.top)")
	initCmd.Flags().BoolVar(&initAllowLan, "allow-lan", false, "Allow LAN connections to the proxy")
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

	dir := configPath

	opts := mihoro.InitOptions{
		Force:    initForce,
		Arch:     initArch,
		AllowLan: initAllowLan,
	}

	return mihoro.RunInit(CliCtx(), newHTTPClient(), dir, opts, initMirror)
}

func newHTTPClient() *http.Client {
	return &http.Client{}
}
