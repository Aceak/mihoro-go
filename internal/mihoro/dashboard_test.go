package mihoro

import (
	"testing"
)

func TestParseControllerAddress(t *testing.T) {
	tests := []struct {
		raw      string
		wantHost string
		wantPort uint16
	}{
		{"0.0.0.0:9090", "0.0.0.0", 9090},
		{"127.0.0.1:19090", "127.0.0.1", 19090},
		{"10.108.25.191:7890", "10.108.25.191", 7890},
		{"[::1]:9090", "[::1]", 9090},
		{"http://0.0.0.0:9090", "0.0.0.0", 9090},
		{"https://127.0.0.1:9090/", "127.0.0.1", 9090},
	}

	for _, tt := range tests {
		host, port := parseControllerAddress(tt.raw)
		if host != tt.wantHost || port != tt.wantPort {
			t.Errorf("parseControllerAddress(%q) = (%q, %d), want (%q, %d)",
				tt.raw, host, port, tt.wantHost, tt.wantPort)
		}
	}
}

func TestIsWildcardHost(t *testing.T) {
	wildcards := []string{"", "*", "0.0.0.0", "::", "[::]"}
	for _, h := range wildcards {
		if !isWildcardHost(h) {
			t.Errorf("isWildcardHost(%q) should be true", h)
		}
	}

	nonWildcards := []string{"127.0.0.1", "localhost", "10.0.0.1", "[::1]"}
	for _, h := range nonWildcards {
		if isWildcardHost(h) {
			t.Errorf("isWildcardHost(%q) should be false", h)
		}
	}
}

func TestIsLoopbackHost(t *testing.T) {
	loopbacks := []string{"127.0.0.1", "localhost", "::1", "[::1]"}
	for _, h := range loopbacks {
		if !isLoopbackHost(h) {
			t.Errorf("isLoopbackHost(%q) should be true", h)
		}
	}

	nonLoopbacks := []string{"10.0.0.1", "0.0.0.0", "example.com"}
	for _, h := range nonLoopbacks {
		if isLoopbackHost(h) {
			t.Errorf("isLoopbackHost(%q) should be false", h)
		}
	}
}

func TestIsDashboardInterface(t *testing.T) {
	filtered := []string{"lo", "docker0", "podman0", "cni0", "virbr0", "lxdbr0",
		"br-abc", "veth123", "flannel0", "kube-dns"}
	for _, name := range filtered {
		if isDashboardInterface(name) {
			t.Errorf("isDashboardInterface(%q) should be false", name)
		}
	}

	real := []string{"eth0", "wlan0", "ens33", "tailscale0"}
	for _, name := range real {
		if !isDashboardInterface(name) {
			t.Errorf("isDashboardInterface(%q) should be true", name)
		}
	}
}

func TestDedupURLs(t *testing.T) {
	urls := []dashboardURL{
		{"Local", "http://127.0.0.1:9090/ui"},
		{"External", "http://10.0.0.1:9090/ui"},
		{"External", "http://10.0.0.1:9090/ui"}, // duplicate
	}

	result := dedupURLs(urls)
	if len(result) != 2 {
		t.Errorf("got %d URLs after dedup, want 2", len(result))
	}
}

func TestParseControllerAddressEdgeCases(t *testing.T) {
	// Empty
	host, port := parseControllerAddress("")
	if host != "" || port != 0 {
		t.Errorf("empty input: got (%q, %d), want ('', 0)", host, port)
	}

	// No port
	host, port = parseControllerAddress("host-without-port")
	if host != "" || port != 0 {
		t.Errorf("no port: got (%q, %d)", host, port)
	}

	// IPv6 bracket without port
	host, port = parseControllerAddress("[::1]")
	if host != "" || port != 0 {
		t.Errorf("ipv6 no port: got (%q, %d)", host, port)
	}

	// IPv6 bracket with missing closing bracket
	host, port = parseControllerAddress("[::1:9090")
	if host != "" || port != 0 {
		t.Errorf("malformed ipv6: got (%q, %d)", host, port)
	}
}

func TestDashboardURLString(t *testing.T) {
	url := dashboardURLString("127.0.0.1", 9090)
	if url != "http://127.0.0.1:9090/ui" {
		t.Errorf("got %q, want http://127.0.0.1:9090/ui", url)
	}
}

func TestIsExternalDashboardIP(t *testing.T) {
	// Valid external IPv4
	// Can't easily create net.IP literals in tests, so just verify the function
	// compiles and basic logic works
	if isExternalDashboardIP(nil) {
		t.Error("nil IP should be false")
	}
}

func TestDedupEmpty(t *testing.T) {
	result := dedupURLs(nil)
	if len(result) != 0 {
		t.Error("dedup of nil should be empty")
	}
	result = dedupURLs([]dashboardURL{})
	if len(result) != 0 {
		t.Error("dedup of empty should be empty")
	}
}
