package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadFromEnvironmentRegistryAuthsFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auths.json")
	require.NoError(t, os.WriteFile(path, []byte(`[{"host":"https://ghcr.io","username":"u","password":"p"}]`), 0o600))
	t.Setenv("IMAGE_TRUST_REGISTRY_AUTHS", "")
	t.Setenv("IMAGE_TRUST_REGISTRY_AUTHS_FILE", path)
	t.Setenv("IMAGE_TRUST_TRUSTED_ISSUERS", "https://token.actions.githubusercontent.com")

	cfg, err := LoadFromEnvironment()
	require.NoError(t, err)
	require.Len(t, cfg.RegistryAuths, 1)
	require.Equal(t, "https://ghcr.io", cfg.RegistryAuths[0].Host)
}

func TestLoadFromEnvironmentRegistryMirrorsFromEnv(t *testing.T) {
	t.Setenv("IMAGE_TRUST_REGISTRY_MIRRORS", "mirror.corp=registry.io,cache=ghcr.io")
	t.Setenv("IMAGE_TRUST_TRUSTED_ISSUERS", "https://token.actions.githubusercontent.com")

	cfg, err := LoadFromEnvironment()
	require.NoError(t, err)
	require.Equal(t, "registry.io", cfg.RegistryMirrors["mirror.corp"])
	require.Equal(t, "ghcr.io", cfg.RegistryMirrors["cache"])
}

func TestLoadFromEnvironmentRegistryMirrorsFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mirrors.txt")
	require.NoError(t, os.WriteFile(path, []byte("mirror.corp=registry.io,cache=ghcr.io"), 0o600))
	t.Setenv("IMAGE_TRUST_REGISTRY_MIRRORS", "")
	t.Setenv("IMAGE_TRUST_REGISTRY_MIRRORS_FILE", path)
	t.Setenv("IMAGE_TRUST_TRUSTED_ISSUERS", "https://token.actions.githubusercontent.com")

	cfg, err := LoadFromEnvironment()
	require.NoError(t, err)
	require.Equal(t, "registry.io", cfg.RegistryMirrors["mirror.corp"])
}

func TestLoadFromEnvironmentRegistryCertDirsFromEnv(t *testing.T) {
	t.Setenv("IMAGE_TRUST_REGISTRY_CERT_DIRS", "registry.example=/certs/a,ghcr.io=/certs/b")
	t.Setenv("IMAGE_TRUST_TRUSTED_ISSUERS", "https://token.actions.githubusercontent.com")

	cfg, err := LoadFromEnvironment()
	require.NoError(t, err)
	require.Equal(t, "/certs/a", cfg.RegistryCertDirs["registry.example"])
	require.Equal(t, "/certs/b", cfg.RegistryCertDirs["ghcr.io"])
}

func TestLoadFromEnvironmentRegistryCertDirsFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cert-dirs.txt")
	require.NoError(t, os.WriteFile(path, []byte("registry.example=/certs/a,ghcr.io=/certs/b"), 0o600))
	t.Setenv("IMAGE_TRUST_REGISTRY_CERT_DIRS", "")
	t.Setenv("IMAGE_TRUST_REGISTRY_CERT_DIRS_FILE", path)
	t.Setenv("IMAGE_TRUST_TRUSTED_ISSUERS", "https://token.actions.githubusercontent.com")

	cfg, err := LoadFromEnvironment()
	require.NoError(t, err)
	require.Equal(t, "/certs/a", cfg.RegistryCertDirs["registry.example"])
}

func TestParseRegistryMirrors(t *testing.T) {
	mirrors, err := parseRegistryMirrors("mirror.corp=registry.io, cache=ghcr.io")
	require.NoError(t, err)
	require.Equal(t, "registry.io", mirrors["mirror.corp"])
	require.Equal(t, "ghcr.io", mirrors["cache"])
}

func TestParseRegistryCertDirs(t *testing.T) {
	dirs, err := parseRegistryCertDirs("registry.example=/certs/a,ghcr.io=/certs/b")
	require.NoError(t, err)
	require.Equal(t, "/certs/a", dirs["registry.example"])
}

func TestLoadFromEnvironmentRegistryCredentials(t *testing.T) {
	dir := t.TempDir()
	passwordPath := filepath.Join(dir, "password")
	require.NoError(t, os.WriteFile(passwordPath, []byte("from-file\n"), 0o600))
	configDir := filepath.Join(dir, "docker")
	require.NoError(t, os.MkdirAll(configDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "config.json"), []byte(`{"auths":{}}`), 0o600))

	t.Setenv("REGISTRY_USER", "robot")
	t.Setenv("REGISTRY_PASSWORD", "")
	t.Setenv("REGISTRY_PASSWORD_FILE", passwordPath)
	t.Setenv("REGISTRY_CERT_DIR", "/certs")
	t.Setenv("REGISTRY_DOCKER_CONFIG_PATH", configDir)
	t.Setenv("IMAGE_TRUST_TRUSTED_ISSUERS", "https://token.actions.githubusercontent.com")

	cfg, err := LoadFromEnvironment()
	require.NoError(t, err)
	require.Equal(t, "robot", cfg.RegistryUser)
	require.Equal(t, "from-file", cfg.RegistryPassword)
	require.Equal(t, "/certs", cfg.RegistryCertDir)
	require.Equal(t, configDir, cfg.RegistryDockerConfigDir)
}
