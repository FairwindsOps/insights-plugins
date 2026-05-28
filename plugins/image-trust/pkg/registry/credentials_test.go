package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadFromEnvironment(t *testing.T) {
	t.Setenv("REGISTRY_USER", "user")
	t.Setenv("REGISTRY_PASSWORD", "pass")
	t.Setenv("REGISTRY_CERT_DIR", "/certs")

	creds, err := LoadFromEnvironment()
	require.NoError(t, err)
	require.Equal(t, "user", creds.Username)
	require.Equal(t, "pass", creds.Password)
	require.Equal(t, "/certs", creds.CertDir)
	require.True(t, creds.Configured())
	require.Equal(t, []string{"--registry-username", "user", "--registry-password", "pass"}, creds.CosignArgs())
	require.Equal(t, []string{"SSL_CERT_DIR=/certs"}, creds.ExtraEnv())
}

func TestLoadFromEnvironmentDockerConfig(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"auths":{}}`), 0o600))

	t.Setenv("REGISTRY_USER", "")
	t.Setenv("REGISTRY_PASSWORD", "")
	t.Setenv("REGISTRY_DOCKER_CONFIG_PATH", dir)

	creds, err := LoadFromEnvironment()
	require.NoError(t, err)
	require.Equal(t, dir, creds.DockerConfigDir)
	require.True(t, creds.Configured())
	require.Empty(t, creds.CosignArgs())
	require.Equal(t, []string{"DOCKER_CONFIG=" + dir}, creds.ExtraEnv())
}

func TestLoadFromEnvironmentPasswordFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "password")
	require.NoError(t, os.WriteFile(path, []byte("from-file\n"), 0o600))

	t.Setenv("REGISTRY_USER", "user")
	t.Setenv("REGISTRY_PASSWORD", "")
	t.Setenv("REGISTRY_PASSWORD_FILE", path)

	creds, err := LoadFromEnvironment()
	require.NoError(t, err)
	require.Equal(t, "from-file", creds.Password)
}
