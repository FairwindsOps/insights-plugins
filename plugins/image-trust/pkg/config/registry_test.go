package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadRegistryAuthsFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auths.json")
	require.NoError(t, os.WriteFile(path, []byte(`[{"host":"https://ghcr.io","username":"u","password":"p"}]`), 0o600))
	t.Setenv("IMAGE_TRUST_REGISTRY_AUTHS", "")
	t.Setenv("IMAGE_TRUST_REGISTRY_AUTHS_FILE", path)

	auths, err := LoadRegistryAuths()
	require.NoError(t, err)
	require.Len(t, auths, 1)
	require.Equal(t, "https://ghcr.io", auths[0].Host)
}

func TestLoadRegistryMirrors(t *testing.T) {
	t.Setenv("IMAGE_TRUST_REGISTRY_MIRRORS", "mirror.corp=registry.io, cache=ghcr.io")
	mirrors, err := LoadRegistryMirrors()
	require.NoError(t, err)
	require.Equal(t, "registry.io", mirrors["mirror.corp"])
	require.Equal(t, "ghcr.io", mirrors["cache"])
}

func TestLoadRegistryCertDirs(t *testing.T) {
	t.Setenv("IMAGE_TRUST_REGISTRY_CERT_DIRS", "registry.example=/certs/a,ghcr.io=/certs/b")
	dirs, err := LoadRegistryCertDirs()
	require.NoError(t, err)
	require.Equal(t, "/certs/a", dirs["registry.example"])
}
