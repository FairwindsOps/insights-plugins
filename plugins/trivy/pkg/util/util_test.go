package util

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldBeAbleToReadOldReports(t *testing.T) {
	v1Body, err := os.ReadFile("testdata/v0.26/latest.json")
	assert.NoError(t, err)

	v2, err := UnmarshalAndFixReport(v1Body)
	assert.NoError(t, err)
	assert.Equal(t, 28, len(v2.Images))
	assert.Equal(t, 467, len(v2.Vulnerabilities))

	for _, img := range v2.Images {
		if img.RecommendationOnly {
			assert.Len(t, img.Owners, 0)
		} else {
			assert.Len(t, img.Owners, 1)
		}
	}
}

func TestUnmarshalAndFixReport(t *testing.T) {
	v2Body, err := os.ReadFile("testdata/v0.27/latest.json")
	assert.NoError(t, err)

	v2, err := UnmarshalAndFixReport(v2Body)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(v2.Images))
	assert.Equal(t, 467, len(v2.Vulnerabilities))

	assert.Len(t, v2.Images[0].Owners, 1)
	assert.Len(t, v2.Images[1].Owners, 1)
	assert.Len(t, v2.Images[2].Owners, 2)
}
