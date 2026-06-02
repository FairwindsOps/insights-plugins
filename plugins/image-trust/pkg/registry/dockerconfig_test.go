package registry

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMergeDockerConfigsOverridesHosts(t *testing.T) {
	first := dockerConfig{
		Auths: map[string]dockerConfigAuth{
			"https://registry.example/v1/": {Username: "old"},
		},
	}
	second := dockerConfig{
		Auths: map[string]dockerConfigAuth{
			"https://registry.example/v1/": {Username: "new"},
		},
	}

	merged := mergeDockerConfigs(first, second)
	require.Equal(t, "new", merged.Auths["https://registry.example/v1/"].Username)
}

func TestDockerConfigWithBasicAuth(t *testing.T) {
	cfg := dockerConfig{}.withBasicAuth("https://index.docker.io/v1/", "robot", "secret")
	require.Equal(t, "robot", cfg.Auths["https://index.docker.io/v1/"].Username)
	require.NotEmpty(t, cfg.Auths["https://index.docker.io/v1/"].Auth)
}
