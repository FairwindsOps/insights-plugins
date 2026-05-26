package config

import (
	"fmt"
	"os"
	"strings"
)

// Config holds runtime configuration for the image-trust plugin.
type Config struct {
	NamespaceAllowlist []string
	NamespaceBlocklist []string
}

// LoadFromEnvironment parses plugin configuration from environment variables.
func LoadFromEnvironment() (*Config, error) {
	cfg := &Config{
		NamespaceAllowlist: parseCSVEnv("NAMESPACE_ALLOWLIST"),
		NamespaceBlocklist: parseCSVEnv("NAMESPACE_BLOCKLIST"),
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate performs basic config validation.
func (c *Config) Validate() error {
	if len(c.NamespaceAllowlist) > 0 && len(c.NamespaceBlocklist) > 0 {
		overlap := make([]string, 0)
		blocked := make(map[string]struct{}, len(c.NamespaceBlocklist))
		for _, ns := range c.NamespaceBlocklist {
			blocked[ns] = struct{}{}
		}
		for _, ns := range c.NamespaceAllowlist {
			if _, ok := blocked[ns]; ok {
				overlap = append(overlap, ns)
			}
		}
		if len(overlap) > 0 {
			return fmt.Errorf("namespaces cannot appear in both allowlist and blocklist: %s", strings.Join(overlap, ", "))
		}
	}
	return nil
}

func parseCSVEnv(name string) []string {
	raw := os.Getenv(name)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(strings.ToLower(part))
		if trimmed == "" {
			continue
		}
		values = append(values, trimmed)
	}
	if len(values) == 0 {
		return nil
	}
	return values
}
