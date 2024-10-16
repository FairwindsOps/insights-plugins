package image

import (
	"fmt"
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/models"
	"github.com/stretchr/testify/assert"
)

var extraFlags = ""
var maxConcurrentScans = 25

func TestScanImages(t *testing.T) {
	scannerMock := func(extraFlags, pullRef string, registryOAuth2AccessToken map[string]string) (*models.TrivyResults, error) {
		return &models.TrivyResults{
			Metadata: models.TrivyMetadata{},
			Results: []models.VulnerabilityList{
				{
					Target:          "target",
					Vulnerabilities: []models.Vulnerability{},
				},
			},
		}, nil
	}
	images := []models.Image{
		{
			Name:    "paulbouwer/hello-kubernetes:1.7",
			PullRef: "paulbouwerhellokubernetes17",
		},
	}
	imgReports := ScanImages(scannerMock, images, maxConcurrentScans, extraFlags, nil)
	assert.NotEmpty(t, imgReports)
	for _, r := range imgReports {
		assert.Len(t, r.Reports, 1)
		assert.Empty(t, r.Error)
	}
}

func TestScanImagesError(t *testing.T) {
	i := 0
	errorScannerMock := func(extraFlags, pullRef string, registryOAuth2AccessToken map[string]string) (*models.TrivyResults, error) {
		i++
		return nil, fmt.Errorf("could not scan image: %d", i)
	}
	images := []models.Image{
		{
			Name:    "paulbouwer/hello-kubernetes:1.7",
			PullRef: "paulbouwerhellokubernetes17",
		},
	}
	imgReports := ScanImages(errorScannerMock, images, maxConcurrentScans, extraFlags, nil)
	assert.NotEmpty(t, imgReports)
	for _, r := range imgReports {
		assert.Len(t, r.Reports, 0)
		assert.Equal(t, "could not scan image: 3", r.Error, "error message should be the last error")
	}
}

func TestScanImagesSuccessOnRetry(t *testing.T) {
	i := 0
	successOnRetryScannerMock := func(extraFlags, pullRef string, registryOAuth2AccessToken map[string]string) (*models.TrivyResults, error) {
		i++
		if i == 3 {
			return &models.TrivyResults{
				Metadata: models.TrivyMetadata{},
				Results: []models.VulnerabilityList{
					{
						Target:          "target",
						Vulnerabilities: []models.Vulnerability{},
					},
				},
			}, nil
		}
		return nil, fmt.Errorf("could not scan image: %d", i)
	}
	images := []models.Image{
		{
			Name:    "paulbouwer/hello-kubernetes:1.7",
			PullRef: "paulbouwerhellokubernetes17",
		},
	}
	imgReports := ScanImages(successOnRetryScannerMock, images, maxConcurrentScans, extraFlags, nil)
	assert.NotEmpty(t, imgReports)
	for _, r := range imgReports {
		assert.Len(t, r.Reports, 1)
		assert.Empty(t, r.Error, "in case of success, error should be empty")
	}
}

func TestScanRecommendationImages(t *testing.T) {
	scannerMock := func(extraFlags, pullRef string, registryOAuth2AccessToken map[string]string) (*models.TrivyResults, error) {
		return &models.TrivyResults{
			Metadata: models.TrivyMetadata{},
			Results: []models.VulnerabilityList{
				{
					Target:          "target",
					Vulnerabilities: []models.Vulnerability{},
				},
			},
		}, nil
	}
	images := []models.Image{
		{
			Name:               "paulbouwer/hello-kubernetes:1.7",
			PullRef:            "paulbouwerhellokubernetes17",
			RecommendationOnly: true,
		},
	}
	imgReports := ScanImages(scannerMock, images, maxConcurrentScans, extraFlags, nil)
	assert.NotEmpty(t, imgReports)
	for _, r := range imgReports {
		assert.Len(t, r.Reports, 1)
		assert.Empty(t, r.Error)
	}
}

func TestScanRecommendationImagesError(t *testing.T) {
	i := 0
	errorScannerMock := func(extraFlags, pullRef string, registryOAuth2AccessToken map[string]string) (*models.TrivyResults, error) {
		i++
		return nil, fmt.Errorf("could not scan image: %d", i)
	}
	images := []models.Image{
		{
			Name:               "paulbouwer/hello-kubernetes:1.7",
			PullRef:            "paulbouwerhellokubernetes17",
			RecommendationOnly: true,
		},
	}
	imgReports := ScanImages(errorScannerMock, images, maxConcurrentScans, extraFlags, nil)
	assert.Empty(t, imgReports, "should not report on errored recommendation only images")
}

func TestScanRecommendationImagesSuccessOnRetry(t *testing.T) {
	i := 0
	successOnRetryScannerMock := func(extraFlags, pullRef string, registryOAuth2AccessToken map[string]string) (*models.TrivyResults, error) {
		i++
		if i == 3 {
			return &models.TrivyResults{
				Metadata: models.TrivyMetadata{},
				Results: []models.VulnerabilityList{
					{
						Target:          "target",
						Vulnerabilities: []models.Vulnerability{},
					},
				},
			}, nil
		}
		return nil, fmt.Errorf("could not scan image: %d", i)
	}
	images := []models.Image{
		{
			Name:               "paulbouwer/hello-kubernetes:1.7",
			PullRef:            "paulbouwerhellokubernetes17",
			RecommendationOnly: true,
		},
	}
	imgReports := ScanImages(successOnRetryScannerMock, images, maxConcurrentScans, extraFlags, nil)
	assert.NotEmpty(t, imgReports)
	for _, r := range imgReports {
		assert.Len(t, r.Reports, 1)
		assert.Empty(t, r.Error, "in case of success, error should be empty")
	}
}
