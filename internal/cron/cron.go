package cron

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// --- crontab path ---

func crontabPath() string {
	runDir := os.Getenv("XDG_RUNTIME_DIR")
	if runDir == "" {
		uid := os.Getuid()
		runDir = fmt.Sprintf("/run/user/%d", uid)
	}
	return filepath.Join(runDir, "mihoro-crontab")
}

// --- cron entry generation ---

func mihoroBinPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("get executable path: %w", err)
	}
	return exe, nil
}

func generateCronEntry(intervalHours uint16) (string, error) {
	binPath, err := mihoroBinPath()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("0 */%d * * * %s update\n", intervalHours, binPath), nil
}

// --- Enable ---

// EnableAutoUpdate installs a crontab entry for periodic mihoro auto-update.
func EnableAutoUpdate(intervalHours uint16, prefix string) error {
	if intervalHours == 0 {
		fmt.Printf("%s Auto-update interval is 0, disabling auto-update\n", prefix)
		return DisableAutoUpdate(prefix)
	}
	if intervalHours > 24 {
		return fmt.Errorf("auto-update interval must be between 1 and 24 hours")
	}

	entry, err := generateCronEntry(intervalHours)
	if err != nil {
		return err
	}

	// Write crontab reference file
	ctabFile := crontabPath()
	if err := os.MkdirAll(filepath.Dir(ctabFile), 0755); err != nil {
		return fmt.Errorf("create crontab dir: %w", err)
	}
	if err := os.WriteFile(ctabFile, []byte(entry), 0644); err != nil {
		return fmt.Errorf("write crontab file: %w", err)
	}

	// Install via crontab command
	cmd := exec.Command("crontab", ctabFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("install crontab: %w\n%s", err, string(out))
	}

	fmt.Printf("%s Auto-update enabled with interval: %d hours\n", prefix, intervalHours)
	fmt.Printf("  -> %s", strings.TrimSpace(entry))
	fmt.Println()
	return nil
}

// --- Disable (safe — removes only mihoro entries, not all crons) ---

// DisableAutoUpdate removes the mihoro crontab entry.
// Unlike the Rust version (crontab -r), this only removes mihoro lines,
// preserving all other user cron jobs.
func DisableAutoUpdate(prefix string) error {
	// Remove reference file
	ctabFile := crontabPath()
	_ = os.Remove(ctabFile)

	// Read current crontab
	current, err := exec.Command("crontab", "-l").Output()
	if err != nil {
		// No crontab at all — already disabled
		fmt.Printf("%s Auto-update disabled (no active cron job)\n", prefix)
		return nil
	}

	// Filter out mihoro entries
	var lines []string
	for _, line := range strings.Split(string(current), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.Contains(trimmed, "mihoro") {
			lines = append(lines, trimmed)
		}
	}

	// Write back filtered crontab
	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(strings.Join(lines, "\n") + "\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("update crontab: %w\n%s", err, string(out))
	}

	fmt.Printf("%s Auto-update disabled\n", prefix)
	return nil
}

// --- Status ---

func formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// Status shows the current auto-update cron status.
func Status(prefix, configPath string) error {
	ctabFile := crontabPath()

	if _, err := os.Stat(ctabFile); os.IsNotExist(err) {
		fmt.Printf("status: Auto-update is disabled\n")
		return nil
	}

	content, err := os.ReadFile(ctabFile)
	if err != nil {
		return fmt.Errorf("read crontab file: %w", err)
	}

	lines := strings.SplitN(string(content), "\n", 2)
	cronEntry := strings.TrimSpace(lines[0])

	fmt.Printf("status: Auto-update is enabled\n")
	fmt.Printf("  -> %s\n", cronEntry)

	// Show last modified time of config file
	if info, err := os.Stat(configPath); err == nil {
		fmt.Printf("  -> Last updated: %s\n", formatTime(info.ModTime()))
	}

	return nil
}
