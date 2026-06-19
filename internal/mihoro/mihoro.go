package mihoro

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mihoro-go/internal/config"
	"mihoro-go/internal/systemctl"
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
// Defined once here — Rust duplicated it in main.rs and init.rs.
type StageReport struct {
	entries []stageResult
}

func NewStageReport() *StageReport {
	return &StageReport{}
}

func (r *StageReport) Begin(name, description string) {
	fmt.Printf("  %s\n", name)
	if description != "" {
		fmt.Printf("  %s\n", description)
	}
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
// If TempFile is non-empty, the binary should be installed from that path.
// If SkipReason is non-empty, the install should be skipped.
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
	Config       *config.Config
	SystemdScope systemctl.SystemdScope
	BinaryPath   string
	ConfigRoot   string
	ConfigPath   string // config.yaml path
	ServicePath  string // mihomo.service path
	Prefix       string
}

// New creates a Mihoro by loading config from path.
// Auto-detects systemd scope based on whether /etc/systemd/system/mihomo.service exists.
func New(configPath string) (*Mihoro, error) {
	cfg, err := config.ParseConfig(ExpandTilde(configPath))
	if err != nil {
		return nil, err
	}

	system := fileExists("/etc/systemd/system/mihomo.service")
	return FromConfig(cfg, system), nil
}

// NewOrDefault loads config from path, falling back to defaults if the config is
// missing or invalid — useful for uninstall where the config may not be fully set up.
func NewOrDefault(configPath string) *Mihoro {
	cfg, err := config.Load(ExpandTilde(configPath))
	if err != nil || cfg == nil {
		c := config.DefaultConfig()
		cfg = &c
	} else if cfg.MihomoBinaryPath == "" || cfg.MihomoConfigRoot == "" || cfg.UserSystemdRoot == "" {
		// Config exists but has empty critical fields — use defaults for paths.
		c := config.DefaultConfig()
		cfg.MihomoBinaryPath = c.MihomoBinaryPath
		cfg.MihomoConfigRoot = c.MihomoConfigRoot
		cfg.UserSystemdRoot = c.UserSystemdRoot
	}
	system := fileExists("/etc/systemd/system/mihomo.service")
	return FromConfig(cfg, system)
}

// FromConfig builds a Mihoro from an already-validated Config.
func FromConfig(cfg *config.Config, system bool) *Mihoro {
	scope := systemctl.UserScope
	serviceRoot := ExpandTilde(cfg.UserSystemdRoot)
	if system {
		scope = systemctl.SystemScope
		serviceRoot = "/etc/systemd/system"
	}

	binaryPath := ExpandTilde(cfg.MihomoBinaryPath)
	configRoot := ExpandTilde(cfg.MihomoConfigRoot)
	configPath := filepath.Join(configRoot, "config.yaml")
	servicePath := filepath.Join(serviceRoot, "mihomo.service")

	return &Mihoro{
		Config:       cfg,
		SystemdScope: scope,
		BinaryPath:   binaryPath,
		ConfigRoot:   configRoot,
		ConfigPath:   configPath,
		ServicePath:  servicePath,
		Prefix:       "mihoro:",
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
