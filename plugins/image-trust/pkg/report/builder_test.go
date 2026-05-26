package report

import (
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestBuild(t *testing.T) {
	results := []models.ImageTrustResult{
		{
			Name: "ghcr.io/example/api:1.0.0",
			ID:   "ghcr.io/example/api@sha256:abc",
			Owners: []models.Resource{
				{Name: "api", Kind: "Deployment", Namespace: "prod", Container: "api"},
			},
			Status: models.StatusUnsigned,
			Reason: "no matching signatures found",
		},
	}

	report := Build(results)

	require.Len(t, report.Images, 1)
	require.Equal(t, models.StatusUnsigned, report.Images[0].Status)
	require.Equal(t, 1, report.Summary.TotalImages)
	require.Equal(t, 1, report.Summary.Unsigned)
	require.Len(t, report.Findings, 1)
	require.Equal(t, "prod", report.Findings[0].ResourceNamespace)
	require.Equal(t, "Container image is unsigned", report.Findings[0].Title)
}

func TestBuildSuppressesAllowlistedFindings(t *testing.T) {
	results := []models.ImageTrustResult{
		{
			Name:            "ghcr.io/example/api:1.0.0",
			ID:              "ghcr.io/example/api@sha256:abc",
			Status:          models.StatusVerificationError,
			Reason:          "context deadline exceeded",
			Allowlisted:     true,
			AllowlistReason: "registry allowlist matched: ghcr.io",
			Owners: []models.Resource{
				{Name: "api", Kind: "Deployment", Namespace: "prod", Container: "api"},
			},
		},
	}

	report := Build(results)

	require.Len(t, report.Images, 1)
	require.Equal(t, 1, report.Summary.VerificationError)
	require.Equal(t, 1, report.Summary.Allowlisted)
	require.Empty(t, report.Findings)
}
