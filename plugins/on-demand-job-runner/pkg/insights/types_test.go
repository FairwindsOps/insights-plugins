package insights

import (
	"slices"
	"testing"
)

func TestOptionsToEnvVars(t *testing.T) {
	job := OnDemandJob{
		ID:         123,
		ReportType: "test-report",
		Status:     string(JobStatusPending),
		Options: map[string]string{
			"jobId":        "123",
			"imagesToScan": "image1:latest,image2:1.0.0",
			"httpRequest":  "GET /api/test",
			"userAPIKey":   "abc123",
		},
	}

	envVars := job.OptionsToEnvVars()

	expected := []string{"JOB_ID=123", "IMAGES_TO_SCAN=image1:latest,image2:1.0.0", "HTTP_REQUEST=GET /api/test", "USER_API_KEY=abc123"}

	for _, envVar := range envVars {
		found := slices.Contains(expected, envVar.Name+"="+envVar.Value)
		if !found {
			t.Errorf("Expected env var %s not found", envVar.Name)
		}
	}
}
