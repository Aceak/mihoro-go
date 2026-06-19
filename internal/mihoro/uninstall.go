package mihoro

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"mihoro-go/internal/systemctl"
	"mihoro-go/internal/utils"
)

func (m *Mihoro) Uninstall(ctx context.Context, mihoroDir string, yes bool) error {
	// Step 1: stop and remove timers (no prompt)
	_ = systemctl.StopTimer(systemctl.SubTimerName)
	_ = systemctl.DisableTimer(systemctl.SubTimerName)
	_ = systemctl.StopTimer(systemctl.UpdateTimerName)
	_ = systemctl.DisableTimer(systemctl.UpdateTimerName)
	_ = os.Remove("/etc/systemd/system/" + systemctl.SubTimerName)
	_ = os.Remove("/etc/systemd/system/" + systemctl.SubServiceName)
	_ = os.Remove("/etc/systemd/system/" + systemctl.UpdateTimerName)
	_ = os.Remove("/etc/systemd/system/" + systemctl.UpdateServiceName)

	fmt.Printf("%s Stopped and removed auto-update timers\n", m.Prefix)

	// Step 2: mihoro config + subscriptions
	if yes {
		_ = os.RemoveAll(mihoroDir)
		fmt.Printf("%s Removed %s\n", m.Prefix, mihoroDir)
	} else {
		ok, err := promptRemove(ctx, m.Prefix, "Remove mihoro config and subscriptions?", mihoroDir)
		if err != nil {
			return err
		}
		if ok {
			_ = os.RemoveAll(mihoroDir)
			fmt.Printf("%s Removed %s\n", m.Prefix, mihoroDir)
		}
	}

	// Step 3: mihoro binary
	if mihoroBin, err := os.Executable(); err == nil {
		if yes {
			removeOrFail(mihoroBin, m.Prefix)
		} else {
			ok, err := promptRemove(ctx, m.Prefix, "Remove mihoro binary?", mihoroBin)
			if err != nil {
				return err
			}
			if ok {
				removeOrFail(mihoroBin, m.Prefix)
			}
		}
	}

	// Step 4: mihomo (default keep)
	if yes {
		_ = systemctl.Stop(systemctl.MihomoService)
		_ = systemctl.Disable(systemctl.MihomoService)
		removeOrFail(m.BinaryPath, m.Prefix)
		removeOrFail(m.ConfigRoot, m.Prefix)
		_ = utils.DeleteFile("/etc/systemd/system/"+systemctl.MihomoService, m.Prefix)
	} else {
		ok, err := promptRemove(ctx, m.Prefix, "Remove mihomo as well?",
			m.BinaryPath, m.ConfigRoot, "/etc/systemd/system/"+systemctl.MihomoService)
		if err != nil {
			return err
		}
		if ok {
			_ = systemctl.Stop(systemctl.MihomoService)
			_ = systemctl.Disable(systemctl.MihomoService)
			removeOrFail(m.BinaryPath, m.Prefix)
			removeOrFail(m.ConfigRoot, m.Prefix)
			_ = utils.DeleteFile("/etc/systemd/system/"+systemctl.MihomoService, m.Prefix)
		}
	}

	_ = systemctl.DaemonReload()

	fmt.Printf("\n%s Uninstall complete\n", m.Prefix)
	return nil
}

func promptRemove(ctx context.Context, prefix, question string, paths ...string) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	fmt.Printf("\n%s %s\n", prefix, question)
	for _, p := range paths {
		fmt.Printf("  > %s\n", p)
	}
	fmt.Print("  Remove? [Y]es / [N]o: ")

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		fmt.Println("  Skipped")
		return false, nil
	}
	text := scanner.Text()
	if len(text) > 0 && (text[0] == 'y' || text[0] == 'Y') {
		return true, nil
	}
	fmt.Println("  Skipped")
	return false, nil
}

func removeOrFail(path, prefix string) {
	if err := os.RemoveAll(path); err != nil {
		fmt.Printf("  %s Failed: %v\n", prefix, err)
	} else {
		fmt.Printf("  %s Removed %s\n", prefix, path)
	}
}
