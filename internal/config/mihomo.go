package config

// MihomoChannel — mihomo release channel.
type MihomoChannel string

const (
	ChannelStable MihomoChannel = "stable"
	ChannelAlpha  MihomoChannel = "alpha"
)

// MihomoMode — proxy mode in mihomo config.yaml.
type MihomoMode string

const (
	ModeGlobal MihomoMode = "global"
	ModeRule   MihomoMode = "rule"
	ModeDirect MihomoMode = "direct"
)

// MihomoLogLevel — log level in mihomo config.yaml.
type MihomoLogLevel string

const (
	LogSilent  MihomoLogLevel = "silent"
	LogError   MihomoLogLevel = "error"
	LogWarning MihomoLogLevel = "warning"
	LogInfo    MihomoLogLevel = "info"
	LogDebug   MihomoLogLevel = "debug"
)

// GeoxUrl — geodata download URLs.
type GeoxUrl struct {
	Geoip   string `yaml:"geoip" toml:"geoip"`
	Geosite string `yaml:"geosite" toml:"geosite"`
	Mmdb    string `yaml:"mmdb" toml:"mmdb"`
}

// MihomoConfig — [mihomo_config] section in mihoro.toml.
// Used to override fields in the remote config.yaml on every apply.
type MihomoConfig struct {
	Port               *uint16  `toml:"port,omitempty"`
	SocksPort          *uint16  `toml:"socks_port,omitempty"`
	MixedPort          *uint16  `toml:"mixed_port,omitempty"`
	RedirPort          *uint16  `toml:"redir_port,omitempty"`
	AllowLan           *bool    `toml:"allow_lan,omitempty"`
	BindAddress        *string  `toml:"bind_address,omitempty"`
	Mode               string   `toml:"mode"`
	LogLevel           string   `toml:"log_level"`
	IPv6               *bool    `toml:"ipv6,omitempty"`
	ExternalController *string  `toml:"external_controller,omitempty"`
	ExternalUI         *string  `toml:"external_ui,omitempty"`
	Secret             *string  `toml:"secret,omitempty"`
	GeodataMode        *bool    `toml:"geodata_mode,omitempty"`
	GeoAutoUpdate      *bool    `toml:"geo_auto_update,omitempty"`
	GeoUpdateInterval  *uint16  `toml:"geo_update_interval,omitempty"`
	GeoxUrl            *GeoxUrl `toml:"geox_url,omitempty"`
}

// DefaultMihomoConfig returns the default MihomoConfig with sensible values.
func DefaultMihomoConfig() MihomoConfig {
	allowLan := false
	bindAddr := "*"
	ipv6 := true
	extController := "0.0.0.0:9090"
	extUI := "ui"
	geodataMode := false
	geoAutoUpdate := true
	geoUpdateInterval := uint16(24)

	return MihomoConfig{
		Port:               ptr(uint16(7891)),
		SocksPort:          ptr(uint16(7892)),
		MixedPort:          ptr(uint16(7890)),
		RedirPort:          nil,
		AllowLan:           &allowLan,
		BindAddress:        &bindAddr,
		Mode:               string(ModeRule),
		LogLevel:           string(LogInfo),
		IPv6:               &ipv6,
		ExternalController: &extController,
		ExternalUI:         &extUI,
		Secret:             nil,
		GeodataMode:        &geodataMode,
		GeoAutoUpdate:      &geoAutoUpdate,
		GeoUpdateInterval:  &geoUpdateInterval,
		GeoxUrl: &GeoxUrl{
			Geoip:   "https://github.com/MetaCubeX/meta-rules-dat/releases/latest/download/geoip.dat",
			Geosite: "https://github.com/MetaCubeX/meta-rules-dat/releases/latest/download/geosite.dat",
			Mmdb:    "https://github.com/MetaCubeX/meta-rules-dat/releases/latest/download/country.mmdb",
		},
	}
}

// MihomoYamlConfig — the structure of mihomo's config.yaml.
// Extra fields (proxies, proxy-groups, rules, etc.) are preserved via yaml:",inline".
type MihomoYamlConfig struct {
	Port               *int     `yaml:"port,omitempty"`
	SocksPort          *int     `yaml:"socks-port,omitempty"`
	MixedPort          *int     `yaml:"mixed-port,omitempty"`
	RedirPort          *int     `yaml:"redir-port,omitempty"`
	AllowLan           *bool    `yaml:"allow-lan,omitempty"`
	BindAddress        *string  `yaml:"bind-address,omitempty"`
	Mode               *string  `yaml:"mode,omitempty"`
	LogLevel           *string  `yaml:"log-level,omitempty"`
	IPv6               *bool    `yaml:"ipv6,omitempty"`
	ExternalController *string  `yaml:"external-controller,omitempty"`
	ExternalUI         *string  `yaml:"external-ui,omitempty"`
	Secret             *string  `yaml:"secret,omitempty"`
	GeodataMode        *bool    `yaml:"geodata-mode,omitempty"`
	GeoAutoUpdate      *bool    `yaml:"geo-auto-update,omitempty"`
	GeoUpdateInterval  *uint16  `yaml:"geo-update-interval,omitempty"`
	GeoxUrl            *GeoxUrl `yaml:"geox-url,omitempty"`

	// Extra preserves all unknown YAML fields (proxies, proxy-groups, rules, etc.).
	Extra map[string]any `yaml:",inline"`
}

// Ui — dashboard source selection.
type Ui string

const (
	UiMetacubexd Ui = "metacubexd"
	UiZashboard  Ui = "zashboard"
	UiYacdMeta   Ui = "yacd-meta"
)

// ParseUi parses a raw string into a Ui value.
// Supports builtin names and "custom:<url>" for custom dashboards.
func ParseUi(raw string) (Ui, error) {
	// custom: prefix
	if url, ok := cutPrefix(raw, "custom:"); ok {
		if url == "" {
			return "", &UiParseError{Raw: raw}
		}
		return Ui(raw), nil
	}
	switch raw {
	case "metacubexd":
		return UiMetacubexd, nil
	case "zashboard":
		return UiZashboard, nil
	case "yacd-meta":
		return UiYacdMeta, nil
	default:
		return "", &UiParseError{Raw: raw}
	}
}

// AsConfigValue returns the canonical config string for this Ui.
func (u Ui) AsConfigValue() string {
	if _, ok := cutPrefix(string(u), "custom:"); ok {
		return string(u)
	}
	switch u {
	case UiMetacubexd:
		return "metacubexd"
	case UiZashboard:
		return "zashboard"
	case UiYacdMeta:
		return "yacd-meta"
	default:
		return string(u)
	}
}

// DownloadURL returns the download URL for the dashboard archive.
func (u Ui) DownloadURL() string {
	switch u {
	case UiMetacubexd:
		return "https://github.com/MetaCubeX/metacubexd/releases/latest/download/compressed-dist.tgz"
	case UiZashboard:
		return "https://github.com/Zephyruso/zashboard/releases/latest/download/dist.zip"
	case UiYacdMeta:
		return "https://github.com/MetaCubeX/Yacd-meta/archive/refs/heads/gh-pages.tar.gz"
	default:
		if url, ok := cutPrefix(string(u), "custom:"); ok {
			return url
		}
		return string(u)
	}
}

func DefaultUi() *Ui {
	u := UiMetacubexd
	return &u
}

// UiParseError is returned when a ui string cannot be parsed.
type UiParseError struct {
	Raw string
}

func (e *UiParseError) Error() string {
	return "unsupported ui: " + e.Raw
}

// --- helpers ---

func ptr[T any](v T) *T { return &v }

// cutPrefix is a small helper until we bump to Go 1.20+ strings.CutPrefix.
func cutPrefix(s, prefix string) (string, bool) {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):], true
	}
	return s, false
}
