package mihoro

import (
	"fmt"

	"mihoro-go/internal/config"
	"mihoro-go/internal/systemctl"
)

// Apply applies config overrides and restarts mihomo.service.
func (m *Mihoro) Apply() error {
	if _, err := config.ApplyOverride(m.ConfigPath, &m.Config.MihomoConfig); err != nil {
		return fmt.Errorf("apply override: %w", err)
	}

	fmt.Printf("%s Applied mihomo config overrides\n", m.Prefix)

	sctl := systemctl.New(m.SystemdScope)
	if err := sctl.Restart("mihomo.service"); err != nil {
		return fmt.Errorf("restart service: %w", err)
	}

	fmt.Printf("%s Restarted mihomo.service\n", m.Prefix)
	return nil
}
