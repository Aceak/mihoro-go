package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config — mihoro top-level configuration (mihoro.toml).
type Config struct {
	RemoteConfigURL       string        `toml:"remote_config_url"`
	UI                    *Ui           `toml:"ui,omitempty"`
	MihomoChannel         MihomoChannel `toml:"mihomo_channel"`
	RemoteMihomoBinaryURL *string       `toml:"remote_mihomo_binary_url,omitempty"`
	MihomoArch            *string       `toml:"mihomo_arch,omitempty"`
	MihomoBinaryPath      string        `toml:"mihomo_binary_path"`
	MihomoConfigRoot      string        `toml:"mihomo_config_root"`
	UserSystemdRoot       string        `toml:"user_systemd_root"`
	MihoroUserAgent       string        `toml:"mihoro_user_agent"`
	AutoUpdateInterval    uint16        `toml:"auto_update_interval"`
	MihomoConfig          MihomoConfig  `toml:"mihomo_config"`
}

// DefaultConfig returns a Config populated with sensible defaults.
func DefaultConfig() Config {
	return Config{
		UI:                 DefaultUi(),
		MihomoChannel:      ChannelStable,
		RemoteConfigURL:    "",
		MihomoBinaryPath:   "~/.local/bin/mihomo",
		MihomoConfigRoot:   "~/.config/mihomo",
		UserSystemdRoot:    "~/.config/systemd/user",
		MihoroUserAgent:    "clash/mihoro-go",
		AutoUpdateInterval: 12,
		MihomoConfig:       DefaultMihomoConfig(),
	}
}

// Load reads a TOML config file from path.  Returns nil if the file does not exist.
func Load(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return &cfg, nil
}

// Save writes the Config to path as TOML, creating parent directories if needed.
func (c *Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create config file %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	if err := toml.NewEncoder(f).Encode(c); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	return nil
}

// WriteDefaultIfMissing writes a default Config to path if the file does not exist.
// Returns true if the file was created.
func WriteDefaultIfMissing(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	}
	cfg := DefaultConfig()
	if err := cfg.Save(path); err != nil {
		return false, err
	}
	return true, nil
}

// ParseConfig loads config from path.  If the file does not exist, creates a
// default config and returns an error directing the user to run `mihoro init`.
// Otherwise validates required fields.
func ParseConfig(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		cfg := DefaultConfig()
		if err := cfg.Save(path); err != nil {
			return nil, fmt.Errorf("create default config: %w", err)
		}
		return nil, fmt.Errorf("created default config at %q, run `mihoro init` to finish setup", path)
	}

	cfg, err := Load(path)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		cfg2 := DefaultConfig()
		cfg = &cfg2
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// validateConfig checks that required Config fields are non-empty.
func validateConfig(cfg *Config) error {
	type req struct {
		name  string
		value string
	}
	for _, r := range []req{
		{"remote_config_url", cfg.RemoteConfigURL},
		{"mihomo_binary_path", cfg.MihomoBinaryPath},
		{"mihomo_config_root", cfg.MihomoConfigRoot},
		{"user_systemd_root", cfg.UserSystemdRoot},
	} {
		if r.value == "" {
			return fmt.Errorf("%q is undefined", r.name)
		}
	}
	return nil
}
