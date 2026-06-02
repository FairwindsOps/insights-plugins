package registry

import (
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestTagReference(t *testing.T) {
	ref := tagReference(models.DiscoveredImage{
		Name: "docker.io/library/nginx:1.25",
	})
	require.Equal(t, "docker.io/library/nginx:1.25", ref)
}

func TestTagReferenceSkipsDigestName(t *testing.T) {
	ref := tagReference(models.DiscoveredImage{
		Name: "ghcr.io/example/api@sha256:abc",
	})
	require.Empty(t, ref)
}
