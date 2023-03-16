package image

import (
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestFullMatchingPipeline(t *testing.T) {
	origReport := []models.ImageDetailsWithRefs{{
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

	inCluster := []models.Image{{
		ID: "quay.io/fairwinds/sample-1@sha256:abcde",
		Name: "quay.io/fairwinds/sample-1:1.2.3",
		RecommendationOnly: false,
	}, {
		ID: "quay.io/fairwinds/sample-2@sha256:12345",
		Name: "quay.io/fairwinds/sample-2:4.5",
		RecommendationOnly: false,
	}}

	toScan := []models.Image{{
		ID: "quay.io/fairwinds/sample-1@sha256:abcde",
		Name: "quay.io/fairwinds/sample-1:1.2.3",
		RecommendationOnly: false,
	}}

	matching := GetMatchingImages(origReport, inCluster, false)
	assert.Equal(t, len(origReport), len(matching))

	matching = GetUnmatchingImages(origReport, toScan, false)
	assert.Equal(t, 3, len(matching))
	assert.Equal(t, "quay.io/fairwinds/sample-2:4.5", matching[0].Name)
}
