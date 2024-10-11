package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadFromEnvironmentDefault(t *testing.T) {
	cfg, err := LoadFromEnvironment()
	assert.NoError(t, err)

	assert.Len(t, cfg.ServiceAccountAnnotations, 0)
	assert.Equal(t, 5, cfg.MaxConcurrentScans)
	assert.Equal(t, 10, cfg.NumberToScan)
	assert.Empty(t, cfg.ExtraFlags)
	assert.False(t, cfg.Offline)
}

func TestLoadFromEnvironment(t *testing.T) {
	err := os.Setenv("SERVICE_ACCOUNT_ANNOTATIONS", `{"key1":"value1","key2":"value2"}`)
	err = os.Setenv("MAX_CONCURRENT_SCANS", "99")
	err = os.Setenv("MAX_SCANS", "88")
	err = os.Setenv("IGNORE_UNFIXED", "true")
	err = os.Setenv("OFFLINE", "true")
	assert.NoError(t, err)
	defer resetEnvVars("SERVICE_ACCOUNT_ANNOTATIONS", "MAX_CONCURRENT_SCANS", "MAX_SCANS", "IGNORE_UNFIXED", "OFFLINE")

	cfg, err := LoadFromEnvironment()
	assert.NoError(t, err)

	assert.Len(t, cfg.ServiceAccountAnnotations, 2)
	assert.Equal(t, 99, cfg.MaxConcurrentScans)
	assert.Equal(t, 88, cfg.NumberToScan)
	assert.Equal(t, "--ignore-unfixed", cfg.ExtraFlags)
	assert.True(t, cfg.Offline)
}

func resetEnvVars(keys ...string) {
	for _, key := range keys {
		os.Unsetenv(key)
	}
}
