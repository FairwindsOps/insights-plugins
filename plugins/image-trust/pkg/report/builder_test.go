package report

import (
	"encoding/json"
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
	require.Contains(t, report.Findings[0].Description, "ghcr.io/example/api@sha256:abc")
	require.Equal(t, findingsCategory, report.Findings[0].Category)
	require.Equal(t, nonCompliantSeverity, report.Findings[0].Severity)

	data, err := json.Marshal(report)
	require.NoError(t, err)
	require.Contains(t, string(data), `"ActionItems"`)
}

func TestBuildVerifiedProducesNoActionItems(t *testing.T) {
	report := Build([]models.ImageTrustResult{
		{
			Name:   "ghcr.io/example/api:1.0.0",
			ID:     "ghcr.io/example/api@sha256:abc",
			Status: models.StatusVerified,
			Owners: []models.Resource{
				{Name: "api", Kind: "Deployment", Namespace: "prod"},
			},
		},
	})

	require.Equal(t, 1, report.Summary.Verified)
	require.Empty(t, report.Findings)
}

func TestBuildSignedUntrustedProducesActionItem(t *testing.T) {
	report := Build([]models.ImageTrustResult{
		{
			Name:   "ghcr.io/example/api:1.0.0",
			ID:     "ghcr.io/example/api@sha256:abc",
			Status: models.StatusSignedUntrusted,
			Reason: "signature was verified but no signer matched the configured trust policy",
			Owners: []models.Resource{
				{Name: "api", Kind: "Deployment", Namespace: "prod"},
			},
		},
	})

	require.Len(t, report.Findings, 1)
	require.Equal(t, "Container image is signed by an untrusted signer", report.Findings[0].Title)
	require.Contains(t, report.Findings[0].Description, "verified a Cosign signature")
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
