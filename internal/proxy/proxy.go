package proxy

import (
	"fmt"
	"net"
	"os"
	"strings"

	"mihoro-go/internal/config"
)

// DetectShell returns "fish" or "bash" based on the SHELL environment variable.
func DetectShell() string {
	if strings.Contains(os.Getenv("SHELL"), "fish") {
		return "fish"
	}
	return "bash" // bash/zsh share export/unset syntax
}

// ExportCmd generates the shell command to set proxy environment variables.
func ExportCmd(shell, host string, port, socksPort uint16) string {
	switch shell {
	case "fish":
		return fmt.Sprintf(
			"set -gx https_proxy http://%s:%d set -gx http_proxy http://%s:%d set -gx all_proxy socks5://%s:%d",
			host, port, host, port, host, socksPort,
		)
	default:
		return fmt.Sprintf(
			"export https_proxy=http://%s:%d http_proxy=http://%s:%d all_proxy=socks5://%s:%d",
			host, port, host, port, host, socksPort,
		)
	}
}

// UnsetCmd generates the shell command to unset proxy environment variables.
func UnsetCmd(shell string) string {
	switch shell {
	case "fish":
		return "set -e https_proxy http_proxy all_proxy"
	default:
		return "unset https_proxy http_proxy all_proxy"
	}
}

// GetPorts returns the HTTP and SOCKS5 proxy ports from a MihomoConfig.
// If MixedPort is set, it takes precedence over Port/SocksPort.
func GetPorts(mc config.MihomoConfig) (port, socksPort uint16) {
	if mc.MixedPort != nil {
		return *mc.MixedPort, *mc.MixedPort
	}
	port = 0
	if mc.Port != nil {
		port = *mc.Port
	}
	socksPort = 0
	if mc.SocksPort != nil {
		socksPort = *mc.SocksPort
	}
	return port, socksPort
}

// LocalIP returns the first non-loopback IPv4 address of this machine,
// falling back to 127.0.0.1 if none is found.
func LocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return "127.0.0.1"
}
