package config

import (
	"fmt"
	"os"
	"reflect"

	"gopkg.in/yaml.v3"
)

// ApplyOverride applies MihomoConfig overrides to a mihomo config.yaml file.
//
// Only the subset of mihomo config fields defined in MihomoConfig are overridden.
// Unrecognized YAML fields (proxies, proxy-groups, rules, etc.) are preserved as-is.
//
// Returns true if the file was modified.
func ApplyOverride(yamlPath string, override *MihomoConfig) (bool, error) {
	raw, err := os.ReadFile(yamlPath)
	if err != nil {
		return false, fmt.Errorf("read yaml %s: %w", yamlPath, err)
	}

	var yml MihomoYamlConfig
	if err := yaml.Unmarshal(raw, &yml); err != nil {
		return false, fmt.Errorf("parse yaml %s: %w", yamlPath, err)
	}

	// Apply every override field onto the YAML struct.
	port := int(override.Port)
	yml.Port = &port
	socksPort := int(override.SocksPort)
	yml.SocksPort = &socksPort
	yml.MixedPort = ptrIntIfNotNil(override.MixedPort)
	yml.RedirPort = ptrIntIfNotNil(override.RedirPort)
	yml.AllowLan = override.AllowLan
	yml.BindAddress = override.BindAddress
	mode := override.Mode
	yml.Mode = &mode
	logLevel := override.LogLevel
	yml.LogLevel = &logLevel
	yml.IPv6 = override.IPv6
	yml.ExternalController = override.ExternalController
	yml.ExternalUI = override.ExternalUI
	yml.Secret = override.Secret
	yml.GeodataMode = override.GeodataMode
	yml.GeoAutoUpdate = override.GeoAutoUpdate
	yml.GeoUpdateInterval = override.GeoUpdateInterval
	yml.GeoxUrl = override.GeoxUrl

	// Serialize and compare with original to avoid unnecessary writes.
	serialized, err := yaml.Marshal(&yml)
	if err != nil {
		return false, fmt.Errorf("marshal yaml: %w", err)
	}

	// Compare as yaml.Node trees to catch semantic equality (ignoring formatting).
	var origNode yaml.Node
	if err := yaml.Unmarshal(raw, &origNode); err != nil {
		return false, fmt.Errorf("parse orig yaml: %w", err)
	}
	var newRawNode yaml.Node
	if err := yaml.Unmarshal(serialized, &newRawNode); err != nil {
		return false, fmt.Errorf("parse new yaml: %w", err)
	}

	if reflect.DeepEqual(&origNode, &newRawNode) {
		return false, nil
	}

	if err := os.WriteFile(yamlPath, serialized, 0644); err != nil {
		return false, fmt.Errorf("write yaml %s: %w", yamlPath, err)
	}
	return true, nil
}

func ptrIntIfNotNil(p *uint16) *int {
	if p == nil {
		return nil
	}
	v := int(*p)
	return &v
}
