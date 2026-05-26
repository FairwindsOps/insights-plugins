package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCSVEnvAndValidation(t *testing.T) {
	t.Setenv("NAMESPACE_ALLOWLIST", "Prod, staging , ,TEAM-A")
	t.Setenv("NAMESPACE_BLOCKLIST", "kube-system")

	cfg, err := LoadFromEnvironment()
	require.NoError(t, err)
	require.Equal(t, []string{"prod", "staging", "team-a"}, cfg.NamespaceAllowlist)
	require.Equal(t, []string{"kube-system"}, cfg.NamespaceBlocklist)
}

func TestValidateRejectsOverlap(t *testing.T) {
	t.Setenv("NAMESPACE_ALLOWLIST", "prod")
	t.Setenv("NAMESPACE_BLOCKLIST", "prod,kube-system")

	_, err := LoadFromEnvironment()
	require.Error(t, err)
	require.Contains(t, err.Error(), "prod")
}
