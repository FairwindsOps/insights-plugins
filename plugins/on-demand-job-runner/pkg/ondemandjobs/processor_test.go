package ondemandjobs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReportTypeJobConfigMapIncludesImageTrust(t *testing.T) {
	cfg, ok := reportTypeJobConfigMap["image-trust"]
	require.True(t, ok)
	require.Equal(t, "image-trust", cfg.cronJobName)
	require.Equal(t, 20*time.Minute, cfg.timeout)
}
