package proxy

import (
	"strings"
	"testing"
)

func TestExportCmdBash(t *testing.T) {
	cmd := ExportCmd("bash", "127.0.0.1", 7891, 7892)

	if !strings.Contains(cmd, "export") {
		t.Error("bash export should contain 'export'")
	}
	if !strings.Contains(cmd, "https_proxy=http://127.0.0.1:7891") {
		t.Error("missing https_proxy")
	}
	if !strings.Contains(cmd, "http_proxy=http://127.0.0.1:7891") {
		t.Error("missing http_proxy")
	}
	if !strings.Contains(cmd, "all_proxy=socks5://127.0.0.1:7892") {
		t.Error("missing all_proxy")
	}
}

func TestExportCmdFish(t *testing.T) {
	cmd := ExportCmd("fish", "10.0.0.1", 8080, 8081)

	if !strings.Contains(cmd, "set -gx") {
		t.Error("fish export should contain 'set -gx'")
	}
	if !strings.Contains(cmd, "https_proxy http://10.0.0.1:8080") {
		t.Error("missing https_proxy")
	}
}

func TestUnsetCmdBash(t *testing.T) {
	cmd := UnsetCmd("bash")
	if !strings.Contains(cmd, "unset") {
		t.Error("bash unset should contain 'unset'")
	}
}

func TestUnsetCmdFish(t *testing.T) {
	cmd := UnsetCmd("fish")
	if !strings.Contains(cmd, "set -e") {
		t.Error("fish unset should contain 'set -e'")
	}
}
