package verify

import (
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/registry"
	"github.com/stretchr/testify/require"
)

func TestPreflightMirrorRemapAllowsVerify(t *testing.T) {
	image := models.DiscoveredImage{
		Name: "mirror.corp/app:1",
		ID:   "registry.io/app@sha256:abc",
	}
	creds := registry.Credentials{
		Mirrors: map[string]string{"mirror.corp": "registry.io"},
	}
	_, stop := Preflight(image, creds)
	require.False(t, stop)
}
