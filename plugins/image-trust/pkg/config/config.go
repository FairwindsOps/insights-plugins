package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultMaxConcurrentScans    = 5
	DefaultImageVerifyTimeout    = 3 * time.Minute
	MaxTrustedSubjectRegexpLen   = 512
	MaxTrustedSubjectRegexpCount = 32
)

// Config holds runtime configuration for the image-trust plugin.
type Config struct {
	NamespaceAllowlist      []string
	NamespaceBlocklist      []string
	VerificationModes       []string
	ModePolicy              string
	TrustedIssuers          []string
	TrustedSubjects         []string
	TrustedSubjectREs       []string
	PublicKeyPaths          []string
	PublicKeyRefs           []string
	PublicKeyDir            string
	TrustedPublicKeys       []TrustedPublicKey
	IgnoreTlog              bool
	SignerAllowlist         []string
	ImageAllowlist          []string
	RegistryAllowlist       []string
	MaxConcurrentScans      int
	ImageVerifyTimeout      time.Duration
	ResolveDigests          bool
	RegistryAuthHost        string
	RegistryAuths           []RegistryAuth
	RegistryMirrors         map[string]string
	RegistryCertDirs        map[string]string
	RegistryUser            string
	RegistryPassword        string
	RegistryCertDir         string
	RegistryDockerConfigDir string
	AttestationTypes        []string
	AttestationsEnabled     bool
	VerifyRetries           int
	VerifyRetryBackoff      time.Duration
	VerifyRetryJitter       bool
	SigstoreEnvFile         string
	SigstoreEnv             []string
}

// LoadFromEnvironment parses plugin configuration from environment variables.
func LoadFromEnvironment() (*Config, error) {
	cfg := &Config{
		NamespaceAllowlist: parseLowerCSVEnv("IMAGE_TRUST_NAMESPACE_ALLOWLIST"),
		NamespaceBlocklist: parseLowerCSVEnv("IMAGE_TRUST_NAMESPACE_BLOCKLIST"),
		VerificationModes:  parseCSVEnv("IMAGE_TRUST_MODES"),
		ModePolicy:         strings.ToLower(strings.TrimSpace(os.Getenv("IMAGE_TRUST_MODE_POLICY"))),
		TrustedIssuers:     parseCSVEnv("IMAGE_TRUST_TRUSTED_ISSUERS"),
		TrustedSubjects:    parseCSVEnv("IMAGE_TRUST_TRUSTED_SUBJECTS"),
		TrustedSubjectREs:  parseCSVEnv("IMAGE_TRUST_TRUSTED_SUBJECT_REGEXPS"),
		AttestationTypes:   parseCSVEnv("IMAGE_TRUST_ATTESTATION_TYPES"),
		PublicKeyPaths:     parseCSVEnv("IMAGE_TRUST_PUBLIC_KEY_PATHS"),
		PublicKeyRefs:      parseCSVEnv("IMAGE_TRUST_PUBLIC_KEY_REFS"),
		PublicKeyDir:       strings.TrimSpace(os.Getenv("IMAGE_TRUST_PUBLIC_KEY_DIR")),
		IgnoreTlog:         parseBoolEnv("IMAGE_TRUST_IGNORE_TLOG"),
		SignerAllowlist:    parseCSVEnv("IMAGE_TRUST_SIGNER_ALLOWLIST"),
		ImageAllowlist:     parseCSVEnv("IMAGE_TRUST_IMAGE_ALLOWLIST"),
		RegistryAllowlist:  parseCSVEnv("IMAGE_TRUST_REGISTRY_ALLOWLIST"),
		MaxConcurrentScans: DefaultMaxConcurrentScans,
		ImageVerifyTimeout: DefaultImageVerifyTimeout,
		ResolveDigests:     true,
		VerifyRetries:      3,
		VerifyRetryBackoff: 2 * time.Second,
		VerifyRetryJitter:  true,
	}
	if len(cfg.VerificationModes) == 0 {
		cfg.VerificationModes = []string{ModeCosignKeyless}
	}
	cfg.VerificationModes = dedupeModes(cfg.VerificationModes)
	if cfg.ModePolicy == "" {
		cfg.ModePolicy = ModePolicyAny
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

	cfg.ResolveDigests = parseBoolEnvDefault("IMAGE_TRUST_RESOLVE_DIGESTS", true)
	cfg.RegistryAuthHost = strings.TrimSpace(os.Getenv("IMAGE_TRUST_REGISTRY_AUTH_HOST"))

	retries := os.Getenv("IMAGE_TRUST_VERIFY_RETRIES")
	if retries != "" {
		value, err := strconv.Atoi(retries)
		if err != nil {
			return nil, fmt.Errorf("parsing IMAGE_TRUST_VERIFY_RETRIES: %w", err)
		}
		if value < 1 {
			return nil, fmt.Errorf("IMAGE_TRUST_VERIFY_RETRIES must be at least 1")
		}
		cfg.VerifyRetries = value
	}

	backoffSeconds := os.Getenv("IMAGE_TRUST_VERIFY_RETRY_BACKOFF_SECONDS")
	if backoffSeconds != "" {
		value, err := strconv.Atoi(backoffSeconds)
		if err != nil {
			return nil, fmt.Errorf("parsing IMAGE_TRUST_VERIFY_RETRY_BACKOFF_SECONDS: %w", err)
		}
		if value < 0 {
			return nil, fmt.Errorf("IMAGE_TRUST_VERIFY_RETRY_BACKOFF_SECONDS must be non-negative")
		}
		cfg.VerifyRetryBackoff = time.Duration(value) * time.Second
	}
	cfg.VerifyRetryJitter = parseBoolEnvDefault("IMAGE_TRUST_VERIFY_RETRY_JITTER", true)
	cfg.AttestationsEnabled = parseBoolEnv("IMAGE_TRUST_ATTESTATIONS_ENABLED")

	cfg.ResolveVerificationModes()

	registry, err := loadRegistrySettings()
	if err != nil {
		return nil, err
	}
	cfg.RegistryAuths = registry.Auths
	cfg.RegistryMirrors = registry.Mirrors
	cfg.RegistryCertDirs = registry.CertDirs
	cfg.RegistryUser = registry.User
	cfg.RegistryPassword = registry.Password
	cfg.RegistryCertDir = registry.CertDir
	cfg.RegistryDockerConfigDir = registry.DockerConfigDir

	sigstoreEnvFile, sigstoreEnv, err := loadSigstoreSettings()
	if err != nil {
		return nil, err
	}
	cfg.SigstoreEnvFile = sigstoreEnvFile
	cfg.SigstoreEnv = sigstoreEnv

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	if modeEnabled(cfg.VerificationModes, ModeCosignKey) {
		keys, err := LoadTrustedPublicKeys(cfg.PublicKeyPaths, cfg.PublicKeyRefs, cfg.PublicKeyDir)
		if err != nil {
			return nil, err
		}
		cfg.TrustedPublicKeys = keys
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
		switch mode {
		case ModeCosignKeyless, ModeCosignKey, ModeCosignAttestationKeyless, ModeCosignAttestationKey:
		default:
			return fmt.Errorf("unsupported verification mode %q", mode)
		}
	}
	switch c.ModePolicy {
	case ModePolicyAny, ModePolicyAll:
	default:
		return fmt.Errorf("unsupported IMAGE_TRUST_MODE_POLICY %q", c.ModePolicy)
	}
	if modeEnabled(c.VerificationModes, ModeCosignKeyless) {
		if len(c.TrustedIssuers) == 0 && len(c.TrustedSubjects) == 0 && len(c.TrustedSubjectREs) == 0 {
			return fmt.Errorf("cosign-keyless requires at least one of IMAGE_TRUST_TRUSTED_ISSUERS, IMAGE_TRUST_TRUSTED_SUBJECTS, or IMAGE_TRUST_TRUSTED_SUBJECT_REGEXPS")
		}
	}
	if modeEnabled(c.VerificationModes, ModeCosignKey) {
		if len(c.PublicKeyPaths) == 0 && len(c.PublicKeyRefs) == 0 && c.PublicKeyDir == "" {
			return fmt.Errorf("cosign-key requires IMAGE_TRUST_PUBLIC_KEY_PATHS, IMAGE_TRUST_PUBLIC_KEY_REFS, or IMAGE_TRUST_PUBLIC_KEY_DIR")
		}
	}
	if modeEnabled(c.VerificationModes, ModeCosignAttestationKeyless) || modeEnabled(c.VerificationModes, ModeCosignAttestationKey) {
		if len(c.AttestationTypes) == 0 {
			return fmt.Errorf("attestation modes require IMAGE_TRUST_ATTESTATION_TYPES")
		}
	}
	if c.wantsAttestations() {
		if len(c.AttestationTypes) == 0 {
			return fmt.Errorf("attestations require IMAGE_TRUST_ATTESTATION_TYPES when IMAGE_TRUST_ATTESTATIONS_ENABLED is true or attestation types are configured")
		}
		if !modeEnabled(c.VerificationModes, ModeCosignAttestationKeyless) && !modeEnabled(c.VerificationModes, ModeCosignAttestationKey) {
			return fmt.Errorf("attestations enabled but no attestation verification mode could be configured; include cosign-keyless and/or cosign-key in IMAGE_TRUST_MODES (matching attestation modes are appended automatically), or set cosign-attestation-keyless and/or cosign-attestation-key explicitly")
		}
	}
	if modeEnabled(c.VerificationModes, ModeCosignAttestationKeyless) {
		if len(c.TrustedIssuers) == 0 && len(c.TrustedSubjects) == 0 && len(c.TrustedSubjectREs) == 0 {
			return fmt.Errorf("cosign-attestation-keyless requires at least one of IMAGE_TRUST_TRUSTED_ISSUERS, IMAGE_TRUST_TRUSTED_SUBJECTS, or IMAGE_TRUST_TRUSTED_SUBJECT_REGEXPS")
		}
	}
	if modeEnabled(c.VerificationModes, ModeCosignAttestationKey) {
		if len(c.PublicKeyPaths) == 0 && len(c.PublicKeyRefs) == 0 && c.PublicKeyDir == "" {
			return fmt.Errorf("cosign-attestation-key requires IMAGE_TRUST_PUBLIC_KEY_PATHS, IMAGE_TRUST_PUBLIC_KEY_REFS, or IMAGE_TRUST_PUBLIC_KEY_DIR")
		}
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

func dedupeModes(modes []string) []string {
	seen := make(map[string]struct{}, len(modes))
	deduped := make([]string, 0, len(modes))
	for _, mode := range modes {
		if _, ok := seen[mode]; ok {
			continue
		}
		seen[mode] = struct{}{}
		deduped = append(deduped, mode)
	}
	return deduped
}

func modeEnabled(modes []string, mode string) bool {
	for _, candidate := range modes {
		if candidate == mode {
			return true
		}
	}
	return false
}

func parseBoolEnv(name string) bool {
	return parseBoolEnvDefault(name, false)
}

func parseBoolEnvDefault(name string, defaultValue bool) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	if raw == "" {
		return defaultValue
	}
	if raw == "0" || raw == "false" || raw == "no" {
		return false
	}
	return raw == "1" || raw == "true" || raw == "yes"
}
