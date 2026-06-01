package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/config"
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
	require.Empty(t, creds.CosignArgs())
}

func TestPrepareMaterializesDockerConfigWithoutCLIPassword(t *testing.T) {
	cfg := &config.Config{
		RegistryAuthHost: "https://index.docker.io/v1/",
		RegistryAuths: []config.RegistryAuth{
			{Host: "https://registry.example/v1/", Username: "robot", Password: "secret"},
		},
	}
	t.Setenv("REGISTRY_USER", "docker-user")
	t.Setenv("REGISTRY_PASSWORD", "docker-pass")

	creds, err := LoadFromEnvironment()
	require.NoError(t, err)

	prepared, err := Prepare(t.Context(), cfg)
	require.NoError(t, err)
	defer prepared.Cleanup()

	require.NotEmpty(t, prepared.Credentials.DockerConfigDir)
	require.Empty(t, prepared.Credentials.Username)
	require.Empty(t, prepared.Credentials.Password)
	require.Empty(t, prepared.Credentials.CosignArgs())

	data, err := os.ReadFile(filepath.Join(prepared.Credentials.DockerConfigDir, "config.json"))
	require.NoError(t, err)
	require.Contains(t, string(data), "registry.example")
	require.Contains(t, string(data), "docker-user")
	_ = creds
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
	require.Empty(t, creds.CosignArgs())
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
