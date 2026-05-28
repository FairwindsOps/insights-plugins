package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func setRequiredTrustPolicyEnv(t *testing.T) {
	t.Helper()
	t.Setenv("IMAGE_TRUST_TRUSTED_ISSUERS", "https://token.actions.githubusercontent.com")
}

func TestParseCSVEnvAndValidation(t *testing.T) {
	setRequiredTrustPolicyEnv(t)
	t.Setenv("NAMESPACE_ALLOWLIST", "Prod, staging , ,TEAM-A")
	t.Setenv("NAMESPACE_BLOCKLIST", "kube-system")
	t.Setenv("IMAGE_TRUST_TRUSTED_SUBJECT_REGEXPS", "https://github.com/example/.+")
	t.Setenv("IMAGE_TRUST_SIGNER_ALLOWLIST", "https://token.actions.githubusercontent.com|https://github.com/example/repo/.github/workflows/build.yml@refs/heads/main")
	t.Setenv("IMAGE_TRUST_IMAGE_ALLOWLIST", "ghcr.io/example/*,docker.io/library/busybox:*")
	t.Setenv("IMAGE_TRUST_REGISTRY_ALLOWLIST", "ghcr.io,*.gcr.io")
	t.Setenv("MAX_CONCURRENT_SCANS", "8")
	t.Setenv("IMAGE_VERIFY_TIMEOUT_SECONDS", "120")

	cfg, err := LoadFromEnvironment()
	require.NoError(t, err)
	require.Equal(t, []string{"prod", "staging", "team-a"}, cfg.NamespaceAllowlist)
	require.Equal(t, []string{"kube-system"}, cfg.NamespaceBlocklist)
	require.Equal(t, []string{"cosign-keyless"}, cfg.VerificationModes)
	require.Equal(t, []string{"https://token.actions.githubusercontent.com"}, cfg.TrustedIssuers)
	require.Equal(t, []string{"https://github.com/example/.+"}, cfg.TrustedSubjectREs)
	require.Equal(t, []string{"https://token.actions.githubusercontent.com|https://github.com/example/repo/.github/workflows/build.yml@refs/heads/main"}, cfg.SignerAllowlist)
	require.Equal(t, []string{"ghcr.io/example/*", "docker.io/library/busybox:*"}, cfg.ImageAllowlist)
	require.Equal(t, []string{"ghcr.io", "*.gcr.io"}, cfg.RegistryAllowlist)
	require.Equal(t, 8, cfg.MaxConcurrentScans)
	require.Equal(t, 120, int(cfg.ImageVerifyTimeout.Seconds()))
}

func TestValidateRejectsOverlap(t *testing.T) {
	setRequiredTrustPolicyEnv(t)
	t.Setenv("NAMESPACE_ALLOWLIST", "prod")
	t.Setenv("NAMESPACE_BLOCKLIST", "prod,kube-system")

	_, err := LoadFromEnvironment()
	require.Error(t, err)
	require.Contains(t, err.Error(), "prod")
}

func TestValidateRejectsUnsupportedMode(t *testing.T) {
	setRequiredTrustPolicyEnv(t)
	t.Setenv("IMAGE_TRUST_MODES", "notation")

	_, err := LoadFromEnvironment()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported verification mode")
}

func TestValidateRequiresTrustPolicyForKeyless(t *testing.T) {
	t.Setenv("IMAGE_TRUST_TRUSTED_ISSUERS", "")
	t.Setenv("IMAGE_TRUST_TRUSTED_SUBJECTS", "")
	t.Setenv("IMAGE_TRUST_TRUSTED_SUBJECT_REGEXPS", "")

	_, err := LoadFromEnvironment()
	require.Error(t, err)
	require.Contains(t, err.Error(), "cosign-keyless requires")
}

func TestValidateCosignKeyRequiresPublicKeys(t *testing.T) {
	t.Setenv("IMAGE_TRUST_MODES", "cosign-key")
	t.Setenv("IMAGE_TRUST_TRUSTED_ISSUERS", "")
	t.Setenv("IMAGE_TRUST_TRUSTED_SUBJECTS", "")
	t.Setenv("IMAGE_TRUST_TRUSTED_SUBJECT_REGEXPS", "")

	_, err := LoadFromEnvironment()
	require.Error(t, err)
	require.Contains(t, err.Error(), "IMAGE_TRUST_PUBLIC_KEY")
}

func TestLoadFromEnvironmentWithBothModes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vendor.pub")
	require.NoError(t, os.WriteFile(path, []byte("public-key"), 0o644))

	t.Setenv("IMAGE_TRUST_MODES", "cosign-keyless,cosign-key")
	t.Setenv("IMAGE_TRUST_TRUSTED_ISSUERS", "https://token.actions.githubusercontent.com")
	t.Setenv("IMAGE_TRUST_PUBLIC_KEY_PATHS", path)

	cfg, err := LoadFromEnvironment()
	require.NoError(t, err)
	require.Equal(t, []string{ModeCosignKeyless, ModeCosignKey}, cfg.VerificationModes)
	require.Equal(t, ModePolicyAny, cfg.ModePolicy)
	require.Len(t, cfg.TrustedPublicKeys, 1)
}

func TestValidateRejectsLongTrustedSubjectRegexp(t *testing.T) {
	setRequiredTrustPolicyEnv(t)
	t.Setenv("IMAGE_TRUST_TRUSTED_SUBJECT_REGEXPS", strings.Repeat("a", MaxTrustedSubjectRegexpLen+1))

	_, err := LoadFromEnvironment()
	require.Error(t, err)
	require.Contains(t, err.Error(), "maximum length")
}
