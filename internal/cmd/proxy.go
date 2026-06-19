package cmd

import (
	"fmt"
	"os"

	"mihoro-go/internal/mihoro"
	"mihoro-go/internal/proxy"

	"github.com/spf13/cobra"
)

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Output proxy export commands",
}

var proxyExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Output proxy export commands (localhost)",
	RunE:  runProxyExport,
}

var proxyExportLanCmd = &cobra.Command{
	Use:   "export-lan",
	Short: "Output proxy export commands (LAN IP)",
	RunE:  runProxyExportLan,
}

var proxyUnsetCmd = &cobra.Command{
	Use:   "unset",
	Short: "Output proxy unset commands",
	RunE:  runProxyUnset,
}

func init() {
	proxyCmd.AddCommand(proxyExportCmd)
	proxyCmd.AddCommand(proxyExportLanCmd)
	proxyCmd.AddCommand(proxyUnsetCmd)
}

func runProxyExport(cmd *cobra.Command, args []string) error {
	m, err := mihoro.New(configPath)
	if err != nil {
		return err
	}
	shell := proxy.DetectShell()
	port, socksPort := proxy.GetPorts(m.Config.MihomoConfig)
	fmt.Println(proxy.ExportCmd(shell, "127.0.0.1", port, socksPort))
	return nil
}

func runProxyExportLan(cmd *cobra.Command, args []string) error {
	m, err := mihoro.New(configPath)
	if err != nil {
		return err
	}

	allowLan := false
	if m.Config.MihomoConfig.AllowLan != nil {
		allowLan = *m.Config.MihomoConfig.AllowLan
	}
	if !allowLan {
		fmt.Fprintf(os.Stderr, "warning: allow_lan is false, proxy is not available for LAN\n")
	}

	ip := proxy.LocalIP()
	shell := proxy.DetectShell()
	port, socksPort := proxy.GetPorts(m.Config.MihomoConfig)
	fmt.Println(proxy.ExportCmd(shell, ip, port, socksPort))
	return nil
}

func runProxyUnset(cmd *cobra.Command, args []string) error {
	shell := proxy.DetectShell()
	fmt.Println(proxy.UnsetCmd(shell))
	return nil
}
