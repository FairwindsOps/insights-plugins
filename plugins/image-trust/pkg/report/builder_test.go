package report

import (
	"testing"
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestBuild(t *testing.T) {
	now := time.Date(2026, time.May, 26, 12, 0, 0, 0, time.UTC)
	images := []models.DiscoveredImage{
		{
			Name: "ghcr.io/example/api:1.0.0",
			ID:   "ghcr.io/example/api@sha256:abc",
			Owners: []models.Resource{
				{Name: "api", Kind: "Deployment", Namespace: "prod", Container: "api"},
			},
		},
	}

	report := Build(images, now)

	require.Len(t, report.Images, 1)
	require.Equal(t, models.StatusUnknown, report.Images[0].Status)
	require.Equal(t, "verification not yet implemented", report.Images[0].Reason)
	require.Equal(t, 1, report.Summary.TotalImages)
	require.Equal(t, 1, report.Summary.Unknown)
	require.Len(t, report.Findings, 1)
	require.Equal(t, "prod", report.Findings[0].ResourceNamespace)
}
