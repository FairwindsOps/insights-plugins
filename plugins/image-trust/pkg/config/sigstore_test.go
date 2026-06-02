package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadFromEnvironmentSigstoreEnvFile(t *testing.T) {
	setRequiredTrustPolicyEnv(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "sigstore.env")
	require.NoError(t, os.WriteFile(path, []byte("CUSTOM_SIGSTORE=1\n"), 0o600))
	t.Setenv("IMAGE_TRUST_SIGSTORE_ENV_FILE", path)
	t.Setenv("FULCIO_URL", "https://fulcio.example")

	cfg, err := LoadFromEnvironment()
	require.NoError(t, err)
	require.Equal(t, path, cfg.SigstoreEnvFile)
	require.Contains(t, cfg.SigstoreEnv, "FULCIO_URL=https://fulcio.example")
	require.Contains(t, cfg.SigstoreEnv, "CUSTOM_SIGSTORE=1")
}
