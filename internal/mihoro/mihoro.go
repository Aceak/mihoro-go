package mihoro

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mihoro-go/internal/config"
	"mihoro-go/internal/systemctl"
	"mihoro-go/internal/version"
)

// ANSI colors, shared across packages.
const (
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Red    = "\033[31m"
	Reset  = "\033[0m"
)

// --- tilde expansion ---

// ExpandTilde replaces a leading "~" with the user's home directory.
func ExpandTilde(path string) string {
	if path == "~" {
		return os.Getenv("HOME")
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(os.Getenv("HOME"), path[2:])
	}
	return path
}

// MihoroDir returns the mihoro config directory (~/.config/mihoro).
func MihoroDir() string {
	return filepath.Join(os.Getenv("HOME"), ".config", "mihoro")
}

// ConfigPath returns the path to config.toml under mihoroDir.
func ConfigPath(mihoroDir string) string {
	return filepath.Join(mihoroDir, "config.toml")
}

// --- StageStatus ---

// StageStatus is the outcome of a single init/update stage.
type StageStatus int

const (
	StageInstalled StageStatus = iota
	StageSkipped
	StageFailed
)

// stageResult holds a human-readable reason alongside the status.
type stageResult struct {
	Name   string
	Status StageStatus
	Reason string // skip reason or error message
}

// --- StageReport ---

// StageReport tracks the outcome of multi-stage operations (init, update).
type StageReport struct {
	entries []stageResult
}

func NewStageReport() *StageReport {
	return &StageReport{}
}

func (r *StageReport) Record(name string, status StageStatus, reason string) {
	r.entries = append(r.entries, stageResult{Name: name, Status: status, Reason: reason})
}

func (r *StageReport) Installed(name string) {
	r.Record(name, StageInstalled, "")
}

func (r *StageReport) Skipped(name, reason string) {
	r.Record(name, StageSkipped, reason)
}

func (r *StageReport) Failed(name string, err error) {
	r.Record(name, StageFailed, err.Error())
}

// HasFailures returns true if any stage failed.
func (r *StageReport) HasFailures() bool {
	for _, e := range r.entries {
		if e.Status == StageFailed {
			return true
		}
	}
	return false
}

// StageFailed returns true if the named stage failed.
func (r *StageReport) StageFailed(name string) bool {
	for _, e := range r.entries {
		if e.Name == name && e.Status == StageFailed {
			return true
		}
	}
	return false
}

// HasInstalled returns true if the named stage completed successfully.
func (r *StageReport) HasInstalled(name string) bool {
	for _, e := range r.entries {
		if e.Name == name && e.Status == StageInstalled {
			return true
		}
	}
	return false
}

// Print outputs the stage summary.
func (r *StageReport) Print(label string) {
	fmt.Printf("mihoro: %s\n", label)
	for _, e := range r.entries {
		switch e.Status {
		case StageInstalled:
			fmt.Printf("  ✓ %s\n", e.Name)
		case StageSkipped:
			fmt.Printf("  ↷ %s (%s)\n", e.Name, e.Reason)
		case StageFailed:
			fmt.Printf("  ✗ %s: %s\n", e.Name, e.Reason)
		}
	}
}

// --- BinaryPlan ---

// BinaryPlan connects Phase 1 (download to temp) and Phase 2 (stop → install).
type BinaryPlan struct {
	TempFile   string // path to downloaded temp file
	SkipReason string // reason to skip installation
}

func (bp BinaryPlan) ShouldInstall() bool {
	return bp.TempFile != ""
}

// --- Mihoro ---

// Mihoro is the core application struct.
type Mihoro struct {
	Config     *config.Config
	Subs       *config.SubscriptionsFile
	BinaryPath string
	ConfigRoot string
	ConfigDir  string // ~/.config/mihoro
	MihomoDir  string // ~/.config/mihomo (mihomo working dir)
	MihomoCfg  string // config.yaml path
	Prefix     string
}

// New creates a Mihoro by loading config from mihoroDir.
func New(mihoroDir string) (*Mihoro, error) {
	checkUpgrade(mihoroDir)

	cfg, err := config.ParseConfig(ConfigPath(mihoroDir))
	if err != nil {
		return nil, err
	}

	subs, err := config.LoadSubscriptions(mihoroDir)
	if err != nil {
		return nil, err
	}

	return FromConfig(cfg, subs, mihoroDir), nil
}

// NewOrDefault loads config from mihoroDir, falling back to defaults.
func NewOrDefault(mihoroDir string) *Mihoro {
	checkUpgrade(mihoroDir)

	cfg, err := config.Load(ConfigPath(mihoroDir))
	if err != nil || cfg == nil {
		c := config.DefaultConfig()
		cfg = &c
	} else if cfg.MihomoBinaryPath == "" || cfg.MihomoConfigRoot == "" {
		c := config.DefaultConfig()
		cfg.MihomoBinaryPath = c.MihomoBinaryPath
		cfg.MihomoConfigRoot = c.MihomoConfigRoot
	}

	subs, _ := config.LoadSubscriptions(mihoroDir)
	if subs == nil {
		subs = &config.SubscriptionsFile{}
	}

	return FromConfig(cfg, subs, mihoroDir)
}

// FromConfig builds a Mihoro from an already-loaded Config and SubscriptionsFile.
func FromConfig(cfg *config.Config, subs *config.SubscriptionsFile, mihoroDir string) *Mihoro {
	binaryPath := ExpandTilde(cfg.MihomoBinaryPath)
	configRoot := ExpandTilde(cfg.MihomoConfigRoot)
	configYaml := filepath.Join(configRoot, "config.yaml")

	return &Mihoro{
		Config:     cfg,
		Subs:       subs,
		BinaryPath: binaryPath,
		ConfigRoot: configRoot,
		ConfigDir:  mihoroDir,
		MihomoDir:  configRoot,
		MihomoCfg:  configYaml,
		Prefix:     "mihoro:",
	}
}

// WriteTimerUnits writes all timer unit files and enables them.
func WriteTimerUnits(mihoroDir, mihoroBin, mirror string) error {
	// subscription timer
	subTimerPath := filepath.Join("/etc/systemd/system", systemctl.SubTimerName)
	subSvcPath := filepath.Join("/etc/systemd/system", systemctl.SubServiceName)
	if err := os.WriteFile(subTimerPath, []byte(systemctl.RenderSubTimer()), 0644); err != nil {
		return fmt.Errorf("write %s: %w", subTimerPath, err)
	}
	if err := os.WriteFile(subSvcPath, []byte(systemctl.RenderSubTimerService(mihoroBin)), 0644); err != nil {
		return fmt.Errorf("write %s: %w", subSvcPath, err)
	}

	// component update timer
	updTimerPath := filepath.Join("/etc/systemd/system", systemctl.UpdateTimerName)
	updSvcPath := filepath.Join("/etc/systemd/system", systemctl.UpdateServiceName)
	if err := os.WriteFile(updTimerPath, []byte(systemctl.RenderUpdateTimer()), 0644); err != nil {
		return fmt.Errorf("write %s: %w", updTimerPath, err)
	}
	if err := os.WriteFile(updSvcPath, []byte(systemctl.RenderUpdateTimerService(mihoroBin, mirror)), 0644); err != nil {
		return fmt.Errorf("write %s: %w", updSvcPath, err)
	}

	if err := systemctl.DaemonReload(); err != nil {
		return err
	}
	if err := systemctl.EnableTimer(systemctl.SubTimerName); err != nil {
		return err
	}
	if err := systemctl.EnableTimer(systemctl.UpdateTimerName); err != nil {
		return err
	}
	if err := systemctl.StartTimer(systemctl.SubTimerName); err != nil {
		return err
	}
	if err := systemctl.StartTimer(systemctl.UpdateTimerName); err != nil {
		return err
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// WriteVersion writes the current mihoro version to the config directory.
func WriteVersion(mihoroDir string) {
	_ = os.WriteFile(filepath.Join(mihoroDir, ".version"), []byte(version.Version), 0644)
}

// checkUpgrade detects version incompatibility and warns the user.
func checkUpgrade(mihoroDir string) {
	oldPath := filepath.Join(os.Getenv("HOME"), ".config", "mihoro.toml")
	newCfgPath := ConfigPath(mihoroDir)

	if !fileExists(oldPath) || fileExists(newCfgPath) {
		return
	}

	fmt.Printf("%swarning:%s detected config from mihoro 0.2.2\n", Yellow, Reset)

	// Try to extract old subscription URL to help user
	data, err := os.ReadFile(oldPath)
	if err != nil {
		return
	}
	oldURL := extractOldURL(string(data))
	if oldURL != "" {
		fmt.Printf("  Old subscription: %s\n", oldURL)
	}
	fmt.Printf("  Run 'mihoro sub add' first, then 'mihoro init --force' to migrate.\n")
	fmt.Printf("  Old config: %s\n", oldPath)
}

func extractOldURL(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "remote_config_url"); ok {
			if _, after, ok := strings.Cut(after, "="); ok {
				return strings.Trim(strings.TrimSpace(after), "\"")
			}
		}
	}
	return ""
}
