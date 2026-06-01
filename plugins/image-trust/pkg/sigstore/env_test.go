package sigstore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadEnvIncludesWellKnownVars(t *testing.T) {
	env, err := LoadEnv(EnvInput{
		Vars: map[string]string{
			"FULCIO_URL": "https://fulcio.example",
			"REKOR_URL":  "https://rekor.example",
		},
	})
	require.NoError(t, err)
	require.Contains(t, env, "FULCIO_URL=https://fulcio.example")
	require.Contains(t, env, "REKOR_URL=https://rekor.example")
}

func TestLoadEnvFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sigstore.env")
	require.NoError(t, os.WriteFile(path, []byte("CUSTOM_SIGSTORE=1\n"), 0o600))

	env, err := LoadEnv(EnvInput{EnvFile: path})
	require.NoError(t, err)
	require.Contains(t, env, "CUSTOM_SIGSTORE=1")
}
