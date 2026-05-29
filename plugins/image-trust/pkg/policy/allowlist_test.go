package policy

import (
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestAllowlistByImagePattern(t *testing.T) {
	matcher := NewAllowlistMatcher([]string{"ghcr.io/example/*"}, nil, nil)
	images := []models.DiscoveredImage{
		{
			Name: "ghcr.io/example/api:1.0.0",
			ID:   "ghcr.io/example/api@sha256:abc",
		},
	}
	results := []models.ImageTrustResult{
		{
			Name:   "ghcr.io/example/api:1.0.0",
			ID:     "ghcr.io/example/api@sha256:abc",
			Status: models.StatusUnsigned,
		},
	}

	updated, err := matcher.Apply(images, results)
	require.NoError(t, err)
	require.True(t, updated[0].Allowlisted)
	require.Contains(t, updated[0].AllowlistReason, "image allowlist matched")
}

func TestAllowlistByRegistryPattern(t *testing.T) {
	matcher := NewAllowlistMatcher(nil, []string{"ghcr.io"}, nil)
	images := []models.DiscoveredImage{
		{
			Name: "ghcr.io/example/api:1.0.0",
			ID:   "ghcr.io/example/api@sha256:abc",
		},
	}
	results := []models.ImageTrustResult{
		{
			Name:   "ghcr.io/example/api:1.0.0",
			ID:     "ghcr.io/example/api@sha256:abc",
			Status: models.StatusVerificationError,
		},
	}

	updated, err := matcher.Apply(images, results)
	require.NoError(t, err)
	require.True(t, updated[0].Allowlisted)
	require.Contains(t, updated[0].AllowlistReason, "registry allowlist matched")
}

func TestAllowlistBySignerPattern(t *testing.T) {
	matcher := NewAllowlistMatcher(nil, nil, []string{"https://token.actions.githubusercontent.com|https://github.com/example/**"})
	images := []models.DiscoveredImage{
		{
			Name: "ghcr.io/example/api:1.0.0",
			ID:   "ghcr.io/example/api@sha256:abc",
		},
	}
	results := []models.ImageTrustResult{
		{
			Name:   "ghcr.io/example/api:1.0.0",
			ID:     "ghcr.io/example/api@sha256:abc",
			Status: models.StatusSignedUntrusted,
			Signer: models.SignerDetails{
				Issuer:  "https://token.actions.githubusercontent.com",
				Subject: "https://github.com/example/repo/.github/workflows/build.yml@refs/heads/main",
			},
			CandidateSigners: []models.SignerDetails{
				{
					Issuer:  "https://token.actions.githubusercontent.com",
					Subject: "https://github.com/example/repo/.github/workflows/build.yml@refs/heads/main",
				},
			},
		},
	}

	updated, err := matcher.Apply(images, results)
	require.NoError(t, err)
	require.True(t, updated[0].Allowlisted)
	require.Contains(t, updated[0].AllowlistReason, "signer allowlist matched")
}

func TestRegistryFromReference(t *testing.T) {
	require.Equal(t, "docker.io", registryFromReference("library/busybox:1.36"))
	require.Equal(t, "ghcr.io", registryFromReference("ghcr.io/example/api:1.0.0"))
	require.Equal(t, "us-docker.pkg.dev", registryFromReference("us-docker.pkg.dev/org/proj/image@sha256:abc"))
}
