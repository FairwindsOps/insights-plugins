package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/config"
	"github.com/stretchr/testify/require"
)

func TestPrepareMaterializesDockerConfigWithoutCLIPassword(t *testing.T) {
	creds := CredentialsFromConfig(&config.Config{
		RegistryAuthHost: "https://index.docker.io/v1/",
		RegistryAuths: []config.RegistryAuth{
			{Host: "https://registry.example/v1/", Username: "robot", Password: "secret"},
		},
		RegistryUser:     "docker-user",
		RegistryPassword: "docker-pass",
	})

	prepared, err := Prepare(t.Context(), creds)
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
	creds := CredentialsFromConfig(&config.Config{
		RegistryUser:     "user",
		RegistryPassword: "pass",
		RegistryCertDir:  "/certs",
		RegistryMirrors:  map[string]string{"mirror.example": "upstream.example"},
		RegistryCertDirs: map[string]string{"ghcr.io": "/certs/ghcr"},
		RegistryAuths: []config.RegistryAuth{
			{Host: "https://ghcr.io", Username: "robot", Password: "token"},
		},
		RegistryAuthHost: "https://index.docker.io/v1/",
	})

	require.Equal(t, "user", creds.Username)
	require.Equal(t, "pass", creds.Password)
	require.Equal(t, "/certs", creds.CertDir)
	require.Len(t, creds.Auths, 1)
	require.Equal(t, "https://index.docker.io/v1/", creds.AuthHost)
	require.True(t, creds.Configured())
}

func TestCredentialsFromConfigNilConfig(t *testing.T) {
	creds := CredentialsFromConfig(nil)
	require.Empty(t, creds.Username)
	require.NotNil(t, creds.Auths)
	require.NotNil(t, creds.Mirrors)
	require.NotNil(t, creds.PerRegistryCertDirs)
}
