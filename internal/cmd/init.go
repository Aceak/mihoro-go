package cmd

import (
	"fmt"
	"net/http"
	"net/url"

	"mihoro-go/internal/bin"
	"mihoro-go/internal/config"
	"mihoro-go/internal/mihoro"
	"mihoro-go/internal/utils"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize mihoro: download binary, config, geodata, and set up systemd service",
	RunE:  runInit,
}

var (
	initForce        bool
	initArch         string
	initMirror       string
	initProxy        string
	initAllowLan     bool
	initDashboardLan bool
	initNoSecret     bool
)

func init() {
	initCmd.Flags().BoolVar(&initForce, "force", false, "Re-download all artifacts even if they already exist")
	initCmd.Flags().StringVar(&initArch, "arch", "", "Override architecture detection")
	initCmd.Flags().StringVar(&initMirror, "mirror", "", "GitHub mirror base URL (e.g. https://ghfast.top)")
	initCmd.Flags().StringVar(&initProxy, "proxy", "",
		"HTTP/SOCKS proxy for downloads (e.g. http://127.0.0.1:7890); saved to config.toml")
	initCmd.Flags().BoolVar(&initAllowLan, "allow-lan", false, "Allow LAN connections to the proxy")
	initCmd.Flags().BoolVar(&initDashboardLan, "dashboard-lan", false,
		"Expose the API/dashboard on 0.0.0.0:9090 (default: 127.0.0.1 only)")
	initCmd.Flags().BoolVar(&initNoSecret, "no-secret", false,
		"Do not create an API secret when none exists (existing secrets are kept)")
}

func runInit(cmd *cobra.Command, args []string) error {
	if initMirror != "" {
		norm, err := utils.NormalizeMirror(initMirror)
		if err != nil {
			return fmt.Errorf("--mirror: %w", err)
		}
		initMirror = norm
	}
	if initArch != "" {
		if _, err := bin.ValidateArch(initArch); err != nil {
			return fmt.Errorf("--arch: %w", err)
		}
	}

	dir := configPath

	// Proxy: the flag wins over config.toml. An explicitly provided proxy is
	// persisted by RunInit so later updates can reuse it.
	proxy, err := utils.NormalizeProxy(initProxy)
	if err != nil {
		return fmt.Errorf("--proxy: %w", err)
	}
	if proxy == "" {
		cfg, cerr := config.Load(mihoro.ConfigPath(dir))
		if cerr != nil {
			fmt.Printf("warning: load config: %v (using no proxy)\n", cerr)
		} else if cfg != nil {
			proxy, err = utils.NormalizeProxy(cfg.Proxy)
			if err != nil {
				return fmt.Errorf("config proxy: %w", err)
			}
		}
	}
	if proxy != "" {
		fmt.Printf("Using proxy %s\n", utils.RedactProxy(proxy))
	}

	opts := mihoro.InitOptions{
		Force:           initForce,
		Arch:            initArch,
		AllowLan:        initAllowLan,
		DashboardLan:    initDashboardLan,
		DashboardLanSet: cmd.Flags().Changed("dashboard-lan"),
		NoSecret:        initNoSecret,
		Proxy:           proxy,
	}

	return mihoro.RunInit(CliCtx(), newHTTPClient(proxy), dir, opts, initMirror)
}

// newHTTPClient returns an HTTP client. A non-empty proxyURL routes every
// request through that proxy; an empty value keeps the default transport,
// which honors HTTP_PROXY/HTTPS_PROXY.
func newHTTPClient(proxyURL string) *http.Client {
	if proxyURL == "" {
		return &http.Client{}
	}
	pu, err := url.Parse(proxyURL)
	if err != nil {
		return &http.Client{}
	}
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Client{}
	}
	tr := base.Clone()
	tr.Proxy = http.ProxyURL(pu)
	return &http.Client{Transport: tr}
}
