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
	subs := &config.SubscriptionsFile{}

	dir := "/tmp/mihoro-test"
	m := FromConfig(&cfg, subs, dir)

	home := os.Getenv("HOME")
	expectedBinary := filepath.Join(home, ".local/bin/mihomo")
	if m.BinaryPath != expectedBinary {
		t.Errorf("BinaryPath = %s, want %s", m.BinaryPath, expectedBinary)
	}
	if m.MihomoCfg != filepath.Join(home, ".config/mihomo/config.yaml") {
		t.Errorf("MihomoCfg = %s", m.MihomoCfg)
	}
	if m.Prefix != "mihoro:" {
		t.Errorf("Prefix = %s", m.Prefix)
	}
	if m.ConfigDir != dir {
		t.Errorf("ConfigDir = %s, want %s", m.ConfigDir, dir)
	}
}

func TestBinaryPlan(t *testing.T) {
	bp := BinaryPlan{TempFile: "/tmp/mihomo-binary"}
	if !bp.ShouldInstall() {
		t.Error("ShouldInstall should be true when TempFile is set")
	}

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

	r.Skipped("skipped stage", "already done")
	if r.StageFailed("skipped stage") {
		t.Error("skipped should not count as failed")
	}

	r.Print("test summary")
}
