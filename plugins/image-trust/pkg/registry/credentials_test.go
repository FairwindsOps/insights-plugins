package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/config"
	"github.com/stretchr/testify/require"
)

func TestPrepareMaterializesDockerConfigWithoutCLIPassword(t *testing.T) {
	cfg := &config.Config{
		RegistryAuthHost: "https://index.docker.io/v1/",
		RegistryAuths: []config.RegistryAuth{
			{Host: "https://registry.example/v1/", Username: "robot", Password: "secret"},
		},
		RegistryUser:     "docker-user",
		RegistryPassword: "docker-pass",
	}

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
}

func TestCredentialsFromConfigUsesInjectedValues(t *testing.T) {
	cfg := &config.Config{
		RegistryUser:     "user",
		RegistryPassword: "pass",
		RegistryCertDir:  "/certs",
		RegistryMirrors:  map[string]string{"mirror.example": "upstream.example"},
		RegistryCertDirs: map[string]string{"ghcr.io": "/certs/ghcr"},
	}

	creds := credentialsFromConfig(cfg)
	require.Equal(t, "user", creds.Username)
	require.Equal(t, "pass", creds.Password)
	require.Equal(t, "/certs", creds.CertDir)
	require.True(t, creds.Configured())
}
