package mihoro

import (
	"fmt"

	"mihoro-go/internal/config"
	"mihoro-go/internal/systemctl"
)

// Apply applies config overrides to the active subscription and restarts mihomo.
func (m *Mihoro) Apply() error {
	activeSub := m.Subs.Active()
	if activeSub == nil {
		return fmt.Errorf("no active subscription")
	}

	subYaml := config.SubDownloadPath(m.ConfigDir, activeSub.Name)

	if err := config.CopyAfterOverride(subYaml, m.MihomoCfg, &m.Config.MihomoConfig); err != nil {
		return fmt.Errorf("apply override: %w", err)
	}

	fmt.Printf("%s Applied overrides\n", m.Prefix)

	if err := systemctl.Restart(systemctl.MihomoService); err != nil {
		return fmt.Errorf("restart service: %w", err)
	}

	fmt.Printf("%s Restarted mihomo.service\n", m.Prefix)
	return nil
}
