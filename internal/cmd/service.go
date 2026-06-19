package cmd

import (
	"fmt"
	"os/exec"

	"mihoro-go/internal/mihoro"
	"mihoro-go/internal/systemctl"

	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start mihomo.service",
	RunE:  serviceAction("start"),
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop mihomo.service",
	RunE:  serviceAction("stop"),
}

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart mihomo.service",
	RunE:  serviceAction("restart"),
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check mihomo.service status",
	RunE:  serviceAction("status"),
}

var logCmd = &cobra.Command{
	Use:     "log",
	Aliases: []string{"logs"},
	Short:   "Check mihomo.service logs with journalctl",
	RunE:    runLog,
}

func runLog(cmd *cobra.Command, args []string) error {
	ctx := CliCtx()

	m, err := mihoro.New(configPath)
	if err != nil {
		return err
	}

	journalArgs := []string{"-xeu", "mihomo.service", "-n", "10", "-f"}
	if m.SystemdScope == systemctl.UserScope {
		journalArgs = append([]string{"--user"}, journalArgs...)
	}

	c := exec.CommandContext(ctx, "journalctl", journalArgs...)
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()
	if err := c.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	return nil
}

func serviceAction(action string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		m, err := mihoro.New(configPath)
		if err != nil {
			return err
		}

		sctl := systemctl.New(m.SystemdScope)
		switch action {
		case "start":
			if err := sctl.Start("mihomo.service"); err != nil {
				return err
			}
			fmt.Printf("%s %sStarted%s mihomo.service\n", m.Prefix, colorGreen, colorReset)
		case "stop":
			if err := sctl.Stop("mihomo.service"); err != nil {
				return err
			}
			fmt.Printf("%s %sStopped%s mihomo.service\n", m.Prefix, colorYellow, colorReset)
		case "restart":
			if err := sctl.Restart("mihomo.service"); err != nil {
				return err
			}
			fmt.Printf("%s %sRestarted%s mihomo.service\n", m.Prefix, colorGreen, colorReset)
		case "status":
			return runStatus(sctl, m)
		}
		return nil
	}
}

func runStatus(sctl *systemctl.Systemctl, m *mihoro.Mihoro) error {
	active := sctl.IsActive("mihomo.service")
	enabled := sctl.IsEnabled("mihomo.service")

	var binVersion string
	if v, err := m.InstalledVersion(); err == nil {
		binVersion = v
	}

	fmt.Printf("%s mihomo.service\n", m.Prefix)
	if binVersion != "" {
		fmt.Printf("  Version:  %s\n", binVersion)
	}
	fmt.Printf("  Binary:   %s\n", m.BinaryPath)
	fmt.Printf("  Config:   %s\n", m.ConfigRoot)

	if active {
		fmt.Printf("  Status:   %sactive%s\n", colorGreen, colorReset)
	} else {
		fmt.Printf("  Status:   %sinactive%s\n", colorRed, colorReset)
	}

	if enabled {
		fmt.Printf("  Boot:     %senabled%s\n", colorGreen, colorReset)
	} else {
		fmt.Printf("  Boot:     %sdisabled%s\n", colorYellow, colorReset)
	}
	return nil
}
