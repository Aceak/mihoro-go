package mihoro

import (
	"os"
	"path/filepath"
	"testing"

	"mihoro-go/internal/config"
)

func TestExpandTilde(t *testing.T) {
	home := os.Getenv("HOME")
	if home == "" {
		t.Skip("HOME not set")
	}

	if got := ExpandTilde("~"); got != home {
		t.Errorf("~ = %s, want %s", got, home)
	}
	if got := ExpandTilde("~/test"); got != home+"/test" {
		t.Errorf("~/test = %s, want %s/test", got, home)
	}
	if got := ExpandTilde("/absolute/path"); got != "/absolute/path" {
		t.Errorf("/absolute/path = %s", got)
	}
}

func TestFromConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.RemoteConfigURL = "http://example.com/config.yaml"

	m := FromConfig(&cfg, false)

	if m.SystemdScope != 0 { // UserScope
		t.Error("expected UserScope")
	}
	home := os.Getenv("HOME")
	expectedBinary := filepath.Join(home, ".local/bin/mihomo")
	if m.BinaryPath != expectedBinary {
		t.Errorf("BinaryPath = %s, want %s", m.BinaryPath, expectedBinary)
	}
	if m.ConfigPath != filepath.Join(home, ".config/mihomo/config.yaml") {
		t.Errorf("ConfigPath = %s", m.ConfigPath)
	}
	if m.Prefix != "mihoro:" {
		t.Errorf("Prefix = %s", m.Prefix)
	}
}

func TestFromConfigSystemScope(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.RemoteConfigURL = "http://example.com/config.yaml"

	m := FromConfig(&cfg, true)

	if m.SystemdScope != 1 { // SystemScope
		t.Error("expected SystemScope")
	}
	if m.ServicePath != "/etc/systemd/system/mihomo.service" {
		t.Errorf("ServicePath = %s, want /etc/systemd/system/mihomo.service", m.ServicePath)
	}
}

func TestBinaryPlan(t *testing.T) {
	// Should install
	bp := BinaryPlan{TempFile: "/tmp/mihomo-binary"}
	if !bp.ShouldInstall() {
		t.Error("ShouldInstall should be true when TempFile is set")
	}

	// Should skip
	bp2 := BinaryPlan{SkipReason: "binary exists"}
	if bp2.ShouldInstall() {
		t.Error("ShouldInstall should be false when only SkipReason is set")
	}
}

func TestStageReport(t *testing.T) {
	r := NewStageReport()

	r.Installed("test stage")
	if r.HasFailures() {
		t.Error("should not have failures")
	}
	if !r.HasInstalled("test stage") {
		t.Error("should have installed")
	}

	r.Failed("failed stage", os.ErrNotExist)
	if !r.HasFailures() {
		t.Error("should have failures after Failed()")
	}
	if !r.StageFailed("failed stage") {
		t.Error("StageFailed should return true")
	}

	r.Skipped("skipped stage", "already done")
	if r.StageFailed("skipped stage") {
		t.Error("skipped should not count as failed")
	}

	// Print should not panic
	r.Print("test summary")
}
