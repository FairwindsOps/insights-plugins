package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultMaxConcurrentScans   = 5
	DefaultImageVerifyTimeout   = 3 * time.Minute
	MaxTrustedSubjectRegexpLen  = 512
	MaxTrustedSubjectRegexpCount = 32
)

// Config holds runtime configuration for the image-trust plugin.
type Config struct {
	NamespaceAllowlist   []string
	NamespaceBlocklist   []string
	VerificationModes    []string
	TrustedIssuers       []string
	TrustedSubjects      []string
	TrustedSubjectREs    []string
	SignerAllowlist      []string
	ImageAllowlist       []string
	RegistryAllowlist    []string
	MaxConcurrentScans   int
	ImageVerifyTimeout   time.Duration
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
		SignerAllowlist:    parseCSVEnv("IMAGE_TRUST_SIGNER_ALLOWLIST"),
		ImageAllowlist:     parseCSVEnv("IMAGE_TRUST_IMAGE_ALLOWLIST"),
		RegistryAllowlist:  parseCSVEnv("IMAGE_TRUST_REGISTRY_ALLOWLIST"),
		MaxConcurrentScans: DefaultMaxConcurrentScans,
		ImageVerifyTimeout: DefaultImageVerifyTimeout,
	}
	if len(cfg.VerificationModes) == 0 {
		cfg.VerificationModes = []string{"cosign-keyless"}
	}

	maxConcurrent := os.Getenv("MAX_CONCURRENT_SCANS")
	if maxConcurrent != "" {
		value, err := strconv.Atoi(maxConcurrent)
		if err != nil {
			return nil, fmt.Errorf("parsing MAX_CONCURRENT_SCANS: %w", err)
		}
		if value < 1 {
			return nil, fmt.Errorf("MAX_CONCURRENT_SCANS must be at least 1")
		}
		cfg.MaxConcurrentScans = value
	}

	timeoutSeconds := os.Getenv("IMAGE_VERIFY_TIMEOUT_SECONDS")
	if timeoutSeconds != "" {
		value, err := strconv.Atoi(timeoutSeconds)
		if err != nil {
			return nil, fmt.Errorf("parsing IMAGE_VERIFY_TIMEOUT_SECONDS: %w", err)
		}
		if value < 1 {
			return nil, fmt.Errorf("IMAGE_VERIFY_TIMEOUT_SECONDS must be at least 1")
		}
		cfg.ImageVerifyTimeout = time.Duration(value) * time.Second
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
	if len(c.TrustedIssuers) == 0 && len(c.TrustedSubjects) == 0 && len(c.TrustedSubjectREs) == 0 {
		return fmt.Errorf("at least one of IMAGE_TRUST_TRUSTED_ISSUERS, IMAGE_TRUST_TRUSTED_SUBJECTS, or IMAGE_TRUST_TRUSTED_SUBJECT_REGEXPS is required")
	}
	if len(c.TrustedSubjectREs) > MaxTrustedSubjectRegexpCount {
		return fmt.Errorf("IMAGE_TRUST_TRUSTED_SUBJECT_REGEXPS supports at most %d patterns", MaxTrustedSubjectRegexpCount)
	}
	for _, pattern := range c.TrustedSubjectREs {
		if len(pattern) > MaxTrustedSubjectRegexpLen {
			return fmt.Errorf("trusted subject regexp exceeds maximum length of %d characters", MaxTrustedSubjectRegexpLen)
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
