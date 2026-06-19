package mihoro

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"mihoro-go/internal/config"
)

// --- types ---

type dashboardURL struct {
	Label string
	URL   string
}

// --- public entry ---

func printDashboardURLs(cfg *config.Config) {
	urls := discoverDashboardURLs(cfg)
	if len(urls) == 0 {
		return
	}

	fmt.Println()
	if len(urls) == 1 && urls[0].Label == "Dashboard" {
		fmt.Printf("Dashboard: %s\n", urls[0].URL)
	} else {
		fmt.Println("Dashboard:")
		for _, u := range urls {
			fmt.Printf("  %s: %s\n", u.Label, u.URL)
		}
	}

	uiName := "metacubexd"
	if cfg.UI != nil {
		uiName = cfg.UI.AsConfigValue()
	}
	fmt.Printf("Using %s - change via the `ui` field in mihoro.toml\n", uiName)

	if cfg.MihomoConfig.Secret != nil && *cfg.MihomoConfig.Secret != "" {
		fmt.Println("Authentication required (secret is set in mihoro.toml)")
	} else {
		fmt.Println("Set `mihomo_config.secret` in mihoro.toml to require a password")
	}
}

// --- discovery ---

func discoverDashboardURLs(cfg *config.Config) []dashboardURL {
	externalUI := cfg.MihomoConfig.ExternalUI
	if externalUI == nil || strings.TrimSpace(*externalUI) == "" {
		return nil
	}

	controller := cfg.MihomoConfig.ExternalController
	if controller == nil {
		return nil
	}

	host, port := parseControllerAddress(*controller)
	if host == "" || port == 0 {
		return nil
	}

	var urls []dashboardURL

	if isWildcardHost(host) {
		urls = append(urls, dashboardURL{"Local", dashboardURLString("127.0.0.1", port)})

		ips := externalDashboardIPs()
		for _, ip := range ips {
			urls = append(urls, dashboardURL{"External", dashboardURLString(ip, port)})
		}
	} else {
		label := "Dashboard"
		if isLoopbackHost(host) {
			label = "Local"
		}
		urls = append(urls, dashboardURL{label, dashboardURLString(host, port)})
	}

	return dedupURLs(urls)
}

func parseControllerAddress(raw string) (host string, port uint16) {
	raw = strings.TrimPrefix(raw, "http://")
	raw = strings.TrimPrefix(raw, "https://")
	raw = strings.TrimSuffix(raw, "/")

	// IPv6 bracket notation: [::1]:9090
	if strings.HasPrefix(raw, "[") {
		idx := strings.Index(raw, "]")
		if idx < 0 {
			return "", 0
		}
		host = raw[:idx+1]
		if !strings.HasPrefix(raw[idx+1:], ":") {
			return "", 0
		}
		portStr := raw[idx+2:]
		p, err := strconv.ParseUint(portStr, 10, 16)
		if err != nil {
			return "", 0
		}
		return host, uint16(p)
	}

	// Standard host:port
	idx := strings.LastIndex(raw, ":")
	if idx < 0 {
		return "", 0
	}
	host = raw[:idx]
	portStr := raw[idx+1:]
	p, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return "", 0
	}
	return host, uint16(p)
}

func dashboardURLString(host string, port uint16) string {
	return fmt.Sprintf("http://%s:%d/ui", host, port)
}

func externalDashboardIPs() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	var ips []string
	for _, iface := range ifaces {
		if !isDashboardInterface(iface.Name) {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip := addrIP(addr)
			if ip != nil && isExternalDashboardIP(ip) {
				ips = append(ips, ip.String())
			}
		}
	}
	return ips
}

func addrIP(addr net.Addr) net.IP {
	switch v := addr.(type) {
	case *net.IPNet:
		return v.IP
	case *net.IPAddr:
		return v.IP
	}
	return nil
}

// isDashboardInterface filters out virtual/container interfaces.
func isDashboardInterface(name string) bool {
	name = strings.ToLower(name)
	switch name {
	case "lo", "docker0", "podman0", "cni0", "virbr0", "lxdbr0":
		return false
	}
	for _, prefix := range []string{"br-", "veth", "flannel", "kube"} {
		if strings.HasPrefix(name, prefix) {
			return false
		}
	}
	return true
}

func isExternalDashboardIP(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		return !ip4.IsLoopback() && !ip4.IsUnspecified() && !ip4.IsLinkLocalUnicast() && !ip4.IsMulticast()
	}
	// Skip IPv6 for now
	return false
}

func isWildcardHost(host string) bool {
	switch strings.TrimSpace(host) {
	case "", "*", "0.0.0.0", "::", "[::]":
		return true
	}
	return false
}

func isLoopbackHost(host string) bool {
	host = strings.TrimSpace(host)

	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		return ip.IsLoopback()
	}
	return strings.EqualFold(host, "localhost")
}

func dedupURLs(urls []dashboardURL) []dashboardURL {
	seen := make(map[string]bool)
	var result []dashboardURL
	for _, u := range urls {
		if !seen[u.URL] {
			seen[u.URL] = true
			result = append(result, u)
		}
	}
	return result
}
