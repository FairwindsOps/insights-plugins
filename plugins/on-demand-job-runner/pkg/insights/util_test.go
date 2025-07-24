package insights

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestToEnvVarFormat(t *testing.T) {
	assert.Equal(t, "JOB_ID", ToEnvVarFormat("jobId"))
	assert.Equal(t, "JOB_ID", ToEnvVarFormat("jobID"))
	assert.Equal(t, "IMAGES_TO_SCAN", ToEnvVarFormat("imagesToScan"))
	assert.Equal(t, "HTTP_REQUEST", ToEnvVarFormat("HTTPRequest"))
	assert.Equal(t, "USER_API_KEY", ToEnvVarFormat("userAPIKey"))

	assert.Equal(t, "JOB_ID", ToEnvVarFormat("JOB_ID"))
	assert.Equal(t, "IMAGES_TO_SCAN", ToEnvVarFormat("IMAGES_TO_SCAN"))
	assert.Equal(t, "HTTP_REQUEST", ToEnvVarFormat("HTTP_REQUEST"))
	assert.Equal(t, "USER_API_KEY", ToEnvVarFormat("USER_API_KEY"))
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
	}{
		{"1h", 1 * time.Hour},
		{"1m", 1 * time.Minute},
		{"1s", 1 * time.Second},
		{"1h5m30s", 1*time.Hour + 5*time.Minute + 30*time.Second},
		{"2h45m30s", 2*time.Hour + 45*time.Minute + 30*time.Second},
		{"1h5m30s200ms", 1*time.Hour + 5*time.Minute + 30*time.Second + 200*time.Millisecond},
		{"2h45m30s200ms", 2*time.Hour + 45*time.Minute + 30*time.Second + 200*time.Millisecond},
		{"1h5m30s200ms400us", 1*time.Hour + 5*time.Minute + 30*time.Second + 200*time.Millisecond + 400*time.Microsecond},
		{"2h45m30s200ms400us", 2*time.Hour + 45*time.Minute + 30*time.Second + 200*time.Millisecond + 400*time.Microsecond},
		{"1h5m30s200ms400us500ns", 1*time.Hour + 5*time.Minute + 30*time.Second + 200*time.Millisecond + 400*time.Microsecond + 500*time.Nanosecond},
		{"1ms", 1 * time.Millisecond},
		{"1us", 1 * time.Microsecond},
		{"1ns", 1 * time.Nanosecond},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			got, err := time.ParseDuration(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}
