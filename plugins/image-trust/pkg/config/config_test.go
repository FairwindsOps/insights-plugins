package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCSVEnvAndValidation(t *testing.T) {
	t.Setenv("NAMESPACE_ALLOWLIST", "Prod, staging , ,TEAM-A")
	t.Setenv("NAMESPACE_BLOCKLIST", "kube-system")
	t.Setenv("IMAGE_TRUST_TRUSTED_ISSUERS", "https://token.actions.githubusercontent.com")
	t.Setenv("IMAGE_TRUST_TRUSTED_SUBJECT_REGEXPS", "https://github.com/example/.+")
	t.Setenv("IMAGE_TRUST_IMAGE_ALLOWLIST", "ghcr.io/example/*,docker.io/library/busybox:*")
	t.Setenv("IMAGE_TRUST_REGISTRY_ALLOWLIST", "ghcr.io,*.gcr.io")

	cfg, err := LoadFromEnvironment()
	require.NoError(t, err)
	require.Equal(t, []string{"prod", "staging", "team-a"}, cfg.NamespaceAllowlist)
	require.Equal(t, []string{"kube-system"}, cfg.NamespaceBlocklist)
	require.Equal(t, []string{"cosign-keyless"}, cfg.VerificationModes)
	require.Equal(t, []string{"https://token.actions.githubusercontent.com"}, cfg.TrustedIssuers)
	require.Equal(t, []string{"https://github.com/example/.+"}, cfg.TrustedSubjectREs)
	require.Equal(t, []string{"ghcr.io/example/*", "docker.io/library/busybox:*"}, cfg.ImageAllowlist)
	require.Equal(t, []string{"ghcr.io", "*.gcr.io"}, cfg.RegistryAllowlist)
}

func TestValidateRejectsOverlap(t *testing.T) {
	t.Setenv("NAMESPACE_ALLOWLIST", "prod")
	t.Setenv("NAMESPACE_BLOCKLIST", "prod,kube-system")

	_, err := LoadFromEnvironment()
	require.Error(t, err)
	require.Contains(t, err.Error(), "prod")
}

func TestValidateRejectsUnsupportedMode(t *testing.T) {
	t.Setenv("IMAGE_TRUST_MODES", "notation")

	_, err := LoadFromEnvironment()
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported verification mode")
}
