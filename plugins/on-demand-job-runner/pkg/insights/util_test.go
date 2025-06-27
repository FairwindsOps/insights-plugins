package insights

import (
	"testing"

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
