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
	VerificationModes  []string
	TrustedIssuers     []string
	TrustedSubjects    []string
	TrustedSubjectREs  []string
	ImageAllowlist     []string
	RegistryAllowlist  []string
}

// LoadFromEnvironment parses plugin configuration from environment variables.
func LoadFromEnvironment() (*Config, error) {
	cfg := &Config{
		NamespaceAllowlist: parseLowerCSVEnv("NAMESPACE_ALLOWLIST"),
		NamespaceBlocklist: parseLowerCSVEnv("NAMESPACE_BLOCKLIST"),
		VerificationModes:  parseCSVEnv("IMAGE_TRUST_MODES"),
		TrustedIssuers:     parseCSVEnv("IMAGE_TRUST_TRUSTED_ISSUERS"),
		TrustedSubjects:    parseCSVEnv("IMAGE_TRUST_TRUSTED_SUBJECTS"),
		TrustedSubjectREs:  parseCSVEnv("IMAGE_TRUST_TRUSTED_SUBJECT_REGEXPS"),
		ImageAllowlist:     parseCSVEnv("IMAGE_TRUST_IMAGE_ALLOWLIST"),
		RegistryAllowlist:  parseCSVEnv("IMAGE_TRUST_REGISTRY_ALLOWLIST"),
	}
	if len(cfg.VerificationModes) == 0 {
		cfg.VerificationModes = []string{"cosign-keyless"}
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
	for _, mode := range c.VerificationModes {
		if mode != "cosign-keyless" {
			return fmt.Errorf("unsupported verification mode %q", mode)
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
		trimmed := strings.TrimSpace(part)
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

func parseLowerCSVEnv(name string) []string {
	values := parseCSVEnv(name)
	if len(values) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		normalized = append(normalized, strings.ToLower(value))
	}
	return normalized
}
