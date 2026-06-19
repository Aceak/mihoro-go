package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"mihoro-go/internal/config"
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
	Use:   "status [core|sub]",
	Short: "Show mihoro and mihomo status. 'status core'=systemctl, 'status sub'=active subscription",
	RunE:  runStatus,
}

var logCmd = &cobra.Command{
	Use:     "log",
	Aliases: []string{"logs"},
	Short:   "Check mihomo.service logs with journalctl",
	RunE:    runLog,
}

func runLog(cmd *cobra.Command, args []string) error {
	ctx := CliCtx()
	journalArgs := []string{"-xeu", "mihomo.service", "-n", "10", "-f"}
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

		switch action {
		case "start":
			if err := systemctl.Start(systemctl.MihomoService); err != nil {
				return err
			}
			fmt.Printf("%s %sStarted%s mihomo.service\n", m.Prefix, colorGreen, colorReset)
		case "stop":
			if err := systemctl.Stop(systemctl.MihomoService); err != nil {
				return err
			}
			fmt.Printf("%s %sStopped%s mihomo.service\n", m.Prefix, colorYellow, colorReset)
		case "restart":
			if err := systemctl.Restart(systemctl.MihomoService); err != nil {
				return err
			}
			fmt.Printf("%s %sRestarted%s mihomo.service\n", m.Prefix, colorGreen, colorReset)
		}
		return nil
	}
}

func runStatus(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "core":
			return runCoreStatus()
		case "sub":
			sf, _, err := loadSubConfig()
			if err != nil {
				return err
			}
			s := sf.Active()
			if s == nil {
				fmt.Println("No active subscription.")
				return nil
			}
			printSubInfo(sf, s)
			return nil
		}
	}

	m, err := mihoro.New(configPath)
	if err != nil {
		return err
	}

	// --- mihomo ---
	fmt.Println("mihomo")
	active := systemctl.IsActive(systemctl.MihomoService)

	if v, vErr := m.InstalledVersion(); vErr == nil && v != "" {
		fmt.Printf("  %-12s %s\n", "Binary:", v)
	}

	geoPresent := checkGeodataPresent(m.ConfigRoot)
	fmt.Printf("  %-12s %s\n", "Geodata:", geoPresent)

	uiPresent := checkUIPresent(m.ConfigRoot, m.Config.MihomoConfig.ExternalUI)
	fmt.Printf("  %-12s %s\n", "UI:", uiPresent)

	if active {
		fmt.Printf("  %-12s %sactive%s\n", "Service:", colorGreen, colorReset)
	} else {
		fmt.Printf("  %-12s %sinactive%s\n", "Service:", colorRed, colorReset)
	}

	// --- auto-update ---
	fmt.Println("auto-update")

	subActive := systemctl.IsActive(systemctl.SubTimerName)
	if subActive {
		next := timerNext(systemctl.SubTimerName)
		fmt.Printf("  %-16s %sactive%s  next: %s\n", "Sub timer:", colorGreen, colorReset, next)
	} else {
		fmt.Printf("  %-16s %sdisabled%s\n", "Sub timer:", colorYellow, colorReset)
	}

	updActive := systemctl.IsActive(systemctl.UpdateTimerName)
	if updActive {
		next := timerNext(systemctl.UpdateTimerName)
		fmt.Printf("  %-16s %sactive%s  next: %s\n", "Comp timer:", colorGreen, colorReset, next)
	} else {
		fmt.Printf("  %-16s %sdisabled%s\n", "Comp timer:", colorYellow, colorReset)
	}

	if m.Config.GitHubMirror != "" {
		fmt.Printf("  %-16s %s\n", "Mirror:", m.Config.GitHubMirror)
	}

	// --- subscription ---
	fmt.Println("subscription")
	activeSub := m.Subs.Active()
	if activeSub == nil {
		fmt.Println("  No active subscription")
	} else {
		lastUpdate := "-"
		if activeSub.LastUpdate != "" {
			lastUpdate = formatSubTime(activeSub.LastUpdate)
		}
		fmt.Printf("  %-12s %s\n", "Active:", activeSub.Name)
		fmt.Printf("  %-12s %s (%s)\n", "Status:", subStatus(activeSub), lastUpdate)
	}

	return nil
}

func runCoreStatus() error {
	unitPath := "/etc/systemd/system/" + systemctl.MihomoService
	if _, err := os.Stat(unitPath); os.IsNotExist(err) {
		fmt.Printf("%sservice not installed%s\n", colorRed, colorReset)
		return nil
	}

	if !systemctl.IsActive(systemctl.MihomoService) {
		fmt.Printf("%sservice not running%s\n", colorRed, colorReset)
	}
	cmd := exec.Command("systemctl", "--no-pager", "status", systemctl.MihomoService)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
	return nil
}

func checkGeodataPresent(configRoot string) string {
	mmdb := configRoot + "/country.mmdb"
	if _, err := os.Stat(mmdb); err == nil {
		return "present"
	}
	geoip := configRoot + "/geoip.dat"
	if _, err := os.Stat(geoip); err == nil {
		return "present"
	}
	return "not present"
}

func checkUIPresent(configRoot string, externalUI *string) string {
	uiDir := configRoot + "/ui"
	if externalUI != nil && *externalUI != "" {
		uiDir = configRoot + "/" + *externalUI
	}
	if _, err := os.Stat(uiDir + "/index.html"); err == nil {
		return "present"
	}
	return "not present"
}

func subStatus(s *config.Subscription) string {
	if s.LastStatus == "success" {
		return fmt.Sprintf("OK (%dKB)", s.LastSize/1024)
	}
	if s.LastStatus == "failed" {
		return "FAILED (" + s.LastError + ")"
	}
	return "never updated"
}

func formatSubTime(t string) string {
	if len(t) >= 16 {
		return t[:10] + " " + t[11:16]
	}
	return t
}

func timerNext(name string) string {
	out, err := exec.Command("systemctl", "show", "--property=NextElapseUSecRealtime", name).Output()
	if err != nil {
		return "-"
	}
	s := strings.TrimSpace(string(out))
	prefix := "NextElapseUSecRealtime="
	after, ok := strings.CutPrefix(s, prefix)
	if !ok {
		return "-"
	}
	// Format: "tue 2026-06-23 01:00:00 CST" or "@1719000000000000"
	after = strings.TrimSpace(after)
	if strings.HasPrefix(after, "@") {
		return after // monotonic timer, can't compute
	}
	// Skip weekday prefix, show YYYY-MM-DD HH:MM
	if len(after) >= 16 {
		// find first digit
		for i, c := range after {
			if c >= '0' && c <= '9' && i+16 <= len(after) {
				return after[i : i+16]
			}
		}
	}
	return after
}
