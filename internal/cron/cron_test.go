package cron

import (
	"os"
	"strings"
	"testing"
)

func TestGenerateCronEntry(t *testing.T) {
	entry, err := generateCronEntry(12)
	if err != nil {
		t.Fatalf("generateCronEntry() = %v", err)
	}

	if !strings.Contains(entry, "0 */12 * * *") {
		t.Errorf("entry should contain cron schedule, got: %s", entry)
	}
	if !strings.Contains(entry, "update") {
		t.Errorf("entry should contain 'update', got: %s", entry)
	}
}

func TestGenerateCronEntryCustomInterval(t *testing.T) {
	entry, err := generateCronEntry(6)
	if err != nil {
		t.Fatalf("generateCronEntry() = %v", err)
	}
	if !strings.Contains(entry, "0 */6 * * *") {
		t.Errorf("entry should contain 6h interval, got: %s", entry)
	}
}

func TestCrontabPath(t *testing.T) {
	path := crontabPath()

	if !strings.HasSuffix(path, "mihoro-crontab") {
		t.Errorf("crontab path should end with mihoro-crontab, got: %s", path)
	}

	// Should be under /run/user/... or XDG_RUNTIME_DIR
	if !strings.Contains(path, "/run/user/") && !strings.Contains(path, os.Getenv("XDG_RUNTIME_DIR")) {
		t.Logf("crontab path: %s", path)
	}
}
