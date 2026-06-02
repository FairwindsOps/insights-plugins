package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVerificationReference(t *testing.T) {
	tests := []struct {
		name  string
		image DiscoveredImage
		want  string
	}{
		{
			name: "full digest id is preserved",
			image: DiscoveredImage{
				Name: "ghcr.io/example/api:1.0.0",
				ID:   "ghcr.io/example/api@sha256:abc",
			},
			want: "ghcr.io/example/api@sha256:abc",
		},
		{
			name: "digest-only id is combined with repository",
			image: DiscoveredImage{
				Name: "ghcr.io/example/api:1.0.0",
				ID:   "sha256:abc",
			},
			want: "ghcr.io/example/api@sha256:abc",
		},
		{
			name: "name digest is used",
			image: DiscoveredImage{
				Name: "ghcr.io/example/api@sha256:abc",
			},
			want: "ghcr.io/example/api@sha256:abc",
		},
		{
			name: "tag only returns empty",
			image: DiscoveredImage{
				Name: "ghcr.io/example/api:1.0.0",
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.image.VerificationReference())
		})
	}
}
