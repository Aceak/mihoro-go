package systemctl

import (
	"fmt"
	"os/exec"
)

// SystemdScope — user or system-level systemd.
type SystemdScope int

const (
	UserScope SystemdScope = iota
	SystemScope
)

// ServiceManager is the interface for managing systemd services.
// The real implementation delegates to systemctl; tests use a mock.
type ServiceManager interface {
	Start(service string) error
	Stop(service string) error
	Restart(service string) error
	Enable(service string) error
	Disable(service string) error
	Status(service string) error
	DaemonReload() error
	IsActive(service string) bool
	IsEnabled(service string) bool
}

// Systemctl is a fluent builder for systemctl commands.
// It implements ServiceManager.
type Systemctl struct {
	scope SystemdScope
}

// New creates a Systemctl with the given scope.
func New(scope SystemdScope) *Systemctl {
	return &Systemctl{scope: scope}
}

// scopeArgs returns --user if user-scoped, empty otherwise.
func (s *Systemctl) scopeArgs() []string {
	if s.scope == UserScope {
		return []string{"--user"}
	}
	return nil
}

func (s *Systemctl) run(args ...string) error {
	cmdArgs := append(s.scopeArgs(), args...)
	cmd := exec.Command("systemctl", cmdArgs...)
	cmd.Stdout = nil // inherit
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("systemctl %v: %w", args, err)
	}
	return nil
}

func (s *Systemctl) isCmd(args ...string) bool {
	cmdArgs := append(s.scopeArgs(), args...)
	return exec.Command("systemctl", cmdArgs...).Run() == nil
}

// --- ServiceManager implementation ---

func (s *Systemctl) Start(service string) error   { return s.run("start", service) }
func (s *Systemctl) Stop(service string) error    { return s.run("stop", service) }
func (s *Systemctl) Restart(service string) error { return s.run("restart", service) }
func (s *Systemctl) Enable(service string) error  { return s.run("enable", service) }
func (s *Systemctl) Disable(service string) error { return s.run("disable", service) }
func (s *Systemctl) Status(service string) error  { return s.run("status", service) }
func (s *Systemctl) DaemonReload() error          { return s.run("daemon-reload") }
func (s *Systemctl) ResetFailed() error           { return s.run("reset-failed") }

func (s *Systemctl) IsActive(service string) bool {
	return s.isCmd("is-active", "--quiet", service)
}

func (s *Systemctl) IsEnabled(service string) bool {
	return s.isCmd("is-enabled", "--quiet", service)
}

// --- systemd unit file template ---

// RenderServiceString generates the content of the mihomo.service unit file.
//
// Reference: https://wiki.metacubex.one/startup/service/
func RenderServiceString(binaryPath, configRoot string, scope SystemdScope) string {
	wantedBy := "multi-user.target"
	if scope == UserScope {
		wantedBy = "default.target"
	}
	return fmt.Sprintf(`[Unit]
Description=mihomo Daemon, Another Clash Kernel.
After=network.target NetworkManager.service systemd-networkd.service iwd.service

[Service]
Type=simple
LimitNPROC=4096
LimitNOFILE=65536
Restart=always
ExecStartPre=/usr/bin/sleep 1s
ExecStart=%s -d %s
ExecReload=/bin/kill -HUP $MAINPID

[Install]
WantedBy=%s
`, binaryPath, configRoot, wantedBy)
}
