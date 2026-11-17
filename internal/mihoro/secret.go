package mihoro

import (
	"fmt"
	"strings"

	"mihoro-go/internal/config"
	"mihoro-go/internal/systemctl"
	"mihoro-go/internal/utils"
)

// secretSource records how the API secret was decided during init.
type secretSource int

const (
	secretKept      secretSource = iota // existing value in config.toml is kept
	secretGenerated                     // a strong secret was generated
	secretInput                         // the user typed a secret
	secretDisabled                      // no secret (API auth left off)
)

// resolveSecret turns a prompt answer into the secret to store.
// An existing secret is always kept. Empty input on a fresh install
// generates one. ok=false means stdin was closed (EOF) before any input.
func resolveSecret(current *string, noSecret, freshInstall bool, line string, ok bool) (*string, secretSource, error) {
	if current != nil && *current != "" {
		return current, secretKept, nil
	}
	if noSecret {
		return nil, secretDisabled, nil
	}
	if !ok && !freshInstall {
		return nil, secretDisabled, nil
	}
	if strings.TrimSpace(line) == "" {
		secret, err := utils.GenerateSecret()
		if err != nil {
			return nil, secretDisabled, err
		}
		return &secret, secretGenerated, nil
	}
	if strings.ContainsAny(line, " \t") {
		return nil, secretDisabled, fmt.Errorf("API secret must not contain whitespace")
	}
	return &line, secretInput, nil
}

// promptSecret asks the user for an API secret unless one already exists.
func promptSecret(p *prompter, current *string, noSecret, freshInstall bool) (*string, secretSource, error) {
	if current != nil && *current != "" {
		return current, secretKept, nil
	}
	if noSecret {
		return nil, secretDisabled, nil
	}
	line, ok := p.Line("API secret (leave empty to generate a strong one)")
	return resolveSecret(current, noSecret, freshInstall, line, ok)
}

// dashboardController computes the external-controller binding from the
// current value and the LAN exposure choice. The port is preserved; a custom
// binding is kept as-is when exposure is declined.
func dashboardController(current *string, exposeLan bool) string {
	port := uint16(9090)
	host := ""
	if current != nil {
		if h, p := parseControllerAddress(*current); p != 0 {
			port = p
			host = h
		}
	}

	if exposeLan {
		if host != "" && isWildcardHost(host) {
			return *current // already bound to all interfaces
		}
		return fmt.Sprintf("0.0.0.0:%d", port)
	}

	if host == "" || isWildcardHost(host) {
		return fmt.Sprintf("127.0.0.1:%d", port)
	}
	return *current // keep loopback and custom bindings as-is
}

// SetSecret stores secret in config.toml and updates the in-memory config.
func (m *Mihoro) SetSecret(secret *string) error {
	m.Config.MihomoConfig.Secret = secret
	if err := m.Config.Save(ConfigPath(m.ConfigDir)); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	return nil
}

// ReapplyOverrides writes the current overrides to config.yaml, using the
// active subscription as source when available and falling back to an
// in-place update of the existing file.
func (m *Mihoro) ReapplyOverrides() error {
	if sub := m.Subs.Active(); sub != nil {
		src := config.SubDownloadPath(m.ConfigDir, sub.Name)
		if fileExists(src) {
			return config.CopyAfterOverride(src, m.MihomoCfg, &m.Config.MihomoConfig)
		}
	}
	if !fileExists(m.MihomoCfg) {
		return fmt.Errorf("config.yaml not found at %s - run `mihoro init` first", m.MihomoCfg)
	}
	_, err := config.ApplyOverride(m.MihomoCfg, &m.Config.MihomoConfig)
	return err
}

// RunSecretReset implements `mihoro secret reset`: it sets (or with disable
// removes) the API secret, reapplies overrides, and restarts mihomo.
func RunSecretReset(mihoroDir string, disable bool) error {
	mihoroDir = ExpandTilde(mihoroDir)

	cfg, err := config.Load(ConfigPath(mihoroDir))
	if err != nil {
		return err
	}
	if cfg == nil {
		return fmt.Errorf("no config at %s - run `mihoro init` first", ConfigPath(mihoroDir))
	}
	if cfg.MihomoConfigRoot == "" {
		return fmt.Errorf("mihomo_config_root is not set in %s - fix it or run `mihoro init`", ConfigPath(mihoroDir))
	}

	subs, err := config.LoadSubscriptions(mihoroDir)
	if err != nil {
		// Rotating the secret does not depend on subscriptions; fall back to
		// an in-place config.yaml update instead of failing.
		fmt.Printf("%swarning:%s load subscriptions: %v\n", Yellow, Reset, err)
		subs = &config.SubscriptionsFile{}
	}
	m := FromConfig(cfg, subs, mihoroDir)

	var newSecret *string
	generated := ""
	if disable {
		newSecret = nil
	} else {
		p := newPrompter()
		line, _ := p.Line("New API secret (leave empty to generate a strong one)")
		if strings.TrimSpace(line) == "" {
			secret, err := utils.GenerateSecret()
			if err != nil {
				return err
			}
			generated = secret
			newSecret = &secret
		} else {
			if strings.ContainsAny(line, " \t") {
				return fmt.Errorf("API secret must not contain whitespace")
			}
			newSecret = &line
		}
	}

	if err := m.SetSecret(newSecret); err != nil {
		return err
	}
	if generated != "" {
		printSecretOnce(generated)
	} else if !disable {
		fmt.Println("API secret: set")
	}

	if err := m.ReapplyOverrides(); err != nil {
		return fmt.Errorf("%w\n  secret saved; fix the issue above, then run `mihoro apply`", err)
	}
	if err := systemctl.Restart(systemctl.MihomoService); err != nil {
		return fmt.Errorf("restart service: %w\n  secret saved; run `mihoro restart`", err)
	}

	printAccessNotice(cfg)
	return nil
}

// printSecretOnce is the only place a secret value is ever displayed to the
// user. It must be called once per generated secret, right after the secret
// has been persisted.
func printSecretOnce(secret string) {
	fmt.Println()
	fmt.Println("============================================================")
	fmt.Println(" API secret (shown only once - save it now):")
	fmt.Printf("   %s\n", secret)
	fmt.Println()
	fmt.Println(" Change it later with: mihoro secret reset")
	fmt.Println("============================================================")
}
