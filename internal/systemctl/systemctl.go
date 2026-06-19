package systemctl

import (
	"fmt"
	"os/exec"
)

// --- service operations ---

func Start(service string) error    { return run("start", service) }
func Stop(service string) error     { return run("stop", service) }
func Restart(service string) error  { return run("restart", service) }
func Enable(service string) error   { return run("enable", service) }
func Disable(service string) error  { return run("disable", service) }
func DaemonReload() error           { return run("daemon-reload") }
func ResetFailed() error            { return run("reset-failed") }

func IsActive(service string) bool  { return isCmd("is-active", "--quiet", service) }
func IsEnabled(service string) bool { return isCmd("is-enabled", "--quiet", service) }

func Status(service string) error { return run("status", service) }

// --- timer operations ---

func StopTimer(name string) error    { return run("stop", name) }
func DisableTimer(name string) error { return run("disable", name) }
func EnableTimer(name string) error  { return run("enable", name) }
func StartTimer(name string) error   { return run("start", name) }

// --- unit file rendering ---

const (
	SubTimerName      = "mihoro-sub.timer"
	SubServiceName    = "mihoro-sub.service"
	UpdateTimerName   = "mihoro-update.timer"
	UpdateServiceName = "mihoro-update.service"
	MihomoService     = "mihomo.service"
)

// RenderMihomoService generates the content of the mihomo.service unit file.
func RenderMihomoService(binaryPath, configRoot string) string {
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
WantedBy=multi-user.target
`, binaryPath, configRoot)
}

// RenderSubTimer generates the subscription auto-update timer unit.
func RenderSubTimer() string {
	return `[Unit]
Description=mihoro subscription auto-update

[Timer]
OnCalendar=*-*-* 02:00:00
RandomizedDelaySec=300

[Install]
WantedBy=timers.target
`
}

// RenderSubTimerService generates the subscription auto-update service unit.
func RenderSubTimerService(mihoroBin string) string {
	return fmt.Sprintf(`[Unit]
Description=mihoro subscription auto-update

[Service]
Type=oneshot
ExecStart=%s sub update --all
`, mihoroBin)
}

// RenderUpdateTimer generates the component auto-update timer unit.
func RenderUpdateTimer() string {
	return `[Unit]
Description=mihoro component auto-update

[Timer]
OnCalendar=Mon *-*-* 01:00:00
RandomizedDelaySec=300

[Install]
WantedBy=timers.target
`
}

// RenderUpdateTimerService generates the component auto-update service unit.
func RenderUpdateTimerService(mihoroBin, mirror string) string {
	cmd := mihoroBin + " update"
	if mirror != "" {
		cmd += " --mirror " + mirror
	}
	return fmt.Sprintf(`[Unit]
Description=mihoro component auto-update

[Service]
Type=oneshot
ExecStart=%s
`, cmd)
}

// --- helpers ---

func run(args ...string) error {
	if err := exec.Command("systemctl", args...).Run(); err != nil {
		return fmt.Errorf("systemctl %v: %w", args, err)
	}
	return nil
}

func isCmd(args ...string) bool {
	return exec.Command("systemctl", args...).Run() == nil
}
