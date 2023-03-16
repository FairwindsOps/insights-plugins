package image

import (
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/models"
	"github.com/stretchr/testify/assert"
)

func getOrigReportForTest() []models.ImageDetailsWithRefs {
	return []models.ImageDetailsWithRefs{{
		ID: "quay.io/fairwinds/sample-1@sha256:abcde",
		Name: "quay.io/fairwinds/sample-1:1.2.3",
		RecommendationOnly: false,
	}, {
		ID: "quay.io/fairwinds/sample-2@sha256:12345",
		Name: "quay.io/fairwinds/sample-2:4.5",
		RecommendationOnly: false,
	}, {
		ID: "quay.io/fairwinds/sample-1@sha256:feg",
		Name: "quay.io/fairwinds/sample-1:2.0.0",
		RecommendationOnly: true,
	}, {
		ID: "quay.io/fairwinds/sample-2@sha256:678",
		Name: "quay.io/fairwinds/sample-2:5.0",
		RecommendationOnly: true,
	}}
}

func TestInClusterMatches(t *testing.T) {
	inClusterAll := []models.Image{{
		ID: "quay.io/fairwinds/sample-1@sha256:abcde",
		Name: "quay.io/fairwinds/sample-1:1.2.3",
		RecommendationOnly: false,
	}, {
		ID: "quay.io/fairwinds/sample-2@sha256:12345",
		Name: "quay.io/fairwinds/sample-2:4.5",
		RecommendationOnly: false,
	}}

	inClusterReduced := []models.Image{{
		ID: "quay.io/fairwinds/sample-2@sha256:12345",
		Name: "quay.io/fairwinds/sample-2:4.5",
		RecommendationOnly: false,
	}}

	matching := GetMatchingImages(getOrigReportForTest(), inClusterAll, false)
	assert.Equal(t, 4, len(matching))

	matching = GetMatchingImages(getOrigReportForTest(), inClusterReduced, false)
	assert.Equal(t, 3, len(matching))
	assert.Equal(t, "quay.io/fairwinds/sample-2:4.5", matching[0].Name)
}

func TestToScanMatches(t *testing.T) {
	toScan := []models.Image{{
		ID: "quay.io/fairwinds/sample-2@sha256:12345",
		Name: "quay.io/fairwinds/sample-2:4.5",
		RecommendationOnly: false,
	}}

	matching := GetUnmatchingImages(getOrigReportForTest(), []models.Image{}, false)
	assert.Equal(t, 4, len(matching))

	matching = GetUnmatchingImages(getOrigReportForTest(), toScan, false)
	assert.Equal(t, 3, len(matching))
	assert.Equal(t, "quay.io/fairwinds/sample-1:1.2.3", matching[0].Name)
}

