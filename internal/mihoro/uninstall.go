package mihoro

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"mihoro-go/internal/cron"
	"mihoro-go/internal/systemctl"
	"mihoro-go/internal/utils"
)

func (m *Mihoro) Uninstall(ctx context.Context, configPath string, yes bool) error {
	sctl := systemctl.New(m.SystemdScope)

	// Always stop and disable the service first.
	_ = sctl.Stop("mihomo.service")
	_ = sctl.Disable("mihomo.service")
	_ = sctl.DaemonReload()
	_ = sctl.ResetFailed()

	fmt.Printf("%s Stopped and disabled mihomo service\n", m.Prefix)

	if err := cron.DisableAutoUpdate(m.Prefix); err != nil {
		return fmt.Errorf("disable cron: %w", err)
	}

	// Step 1: mihoro binary itself.
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

	// Step 2: mihoro config (mihoro.toml).
	if yes {
		_ = utils.DeleteFile(configPath, m.Prefix)
	} else {
		ok, err := promptRemove(ctx, m.Prefix, "Remove mihoro config?", configPath)
		if err != nil {
			return err
		}
		if ok {
			_ = utils.DeleteFile(configPath, m.Prefix)
		}
	}

	// Step 3: mihomo related files (binary, config directory, service).
	if yes {
		removeOrFail(m.BinaryPath, m.Prefix)
		removeOrFail(m.ConfigRoot, m.Prefix)
		_ = utils.DeleteFile(m.ServicePath, m.Prefix)
	} else {
		ok, err := promptRemove(ctx, m.Prefix, "Remove mihomo binary, config and service?",
			m.BinaryPath, m.ConfigRoot, m.ServicePath+"  (service)")
		if err != nil {
			return err
		}
		if ok {
			removeOrFail(m.BinaryPath, m.Prefix)
			removeOrFail(m.ConfigRoot, m.Prefix)
			_ = utils.DeleteFile(m.ServicePath, m.Prefix)
		}
	}

	fmt.Printf("\n%s Uninstall complete\n", m.Prefix)
	return nil
}

// promptRemove asks the user a yes/no question with the file paths shown.
// Returns (true, nil) for yes, (false, nil) for no/skip.
// Returns (false, err) if the context is cancelled during input.
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
		fmt.Printf("  Removed %s\n", path)
	}
}
