package sigstore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadFromEnvironmentIncludesWellKnownVars(t *testing.T) {
	t.Setenv("FULCIO_URL", "https://fulcio.example")
	t.Setenv("REKOR_URL", "https://rekor.example")

	env, err := LoadFromEnvironment()
	require.NoError(t, err)
	require.Contains(t, env, "FULCIO_URL=https://fulcio.example")
	require.Contains(t, env, "REKOR_URL=https://rekor.example")
}

func TestLoadFromEnvironmentFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sigstore.env")
	require.NoError(t, os.WriteFile(path, []byte("CUSTOM_SIGSTORE=1\n"), 0o600))
	t.Setenv("IMAGE_TRUST_SIGSTORE_ENV_FILE", path)

	env, err := LoadFromEnvironment()
	require.NoError(t, err)
	require.Contains(t, env, "CUSTOM_SIGSTORE=1")
}
