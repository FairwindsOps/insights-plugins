package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadFromEnvironmentDefault(t *testing.T) {
	cfg, err := LoadFromEnvironment()
	assert.NoError(t, err)

	assert.False(t, cfg.HasGKESAAnnotation)
	assert.Empty(t, cfg.NamespaceBlocklist)
	assert.Empty(t, cfg.NamespaceAllowlist)
	assert.Equal(t, 5, cfg.MaxConcurrentScans)
	assert.Equal(t, 10, cfg.NumberToScan)
	assert.Empty(t, cfg.ExtraFlags)
	assert.False(t, cfg.Offline)
}

func TestLoadFromEnvironment(t *testing.T) {
	err := os.Setenv("SERVICE_ACCOUNT_ANNOTATIONS", `{"iam.gke.io/gcp-service-account":"my-gsa@my-project.iam.gserviceaccount.com","another-key":"another-value"}`)
	assert.NoError(t, err)
	err = os.Setenv("MAX_CONCURRENT_SCANS", "99")
	assert.NoError(t, err)
	err = os.Setenv("MAX_SCANS", "88")
	assert.NoError(t, err)
	err = os.Setenv("IGNORE_UNFIXED", "true")
	assert.NoError(t, err)
	err = os.Setenv("OFFLINE", "true")
	assert.NoError(t, err)
	err = os.Setenv("NAMESPACE_BLOCKLIST", "kube-system,kube-public")
	assert.NoError(t, err)
	err = os.Setenv("NAMESPACE_ALLOWLIST", "default,fw-insights")
	assert.NoError(t, err)
	defer resetEnvVars("SERVICE_ACCOUNT_ANNOTATIONS", "MAX_CONCURRENT_SCANS", "MAX_SCANS", "IGNORE_UNFIXED", "OFFLINE", "NAMESPACE_BLOCKLIST", "NAMESPACE_ALLOWLIST")

	cfg, err := LoadFromEnvironment()
	assert.NoError(t, err)

	assert.True(t, cfg.HasGKESAAnnotation, 1)
	assert.Len(t, cfg.NamespaceBlocklist, 2)
	assert.Len(t, cfg.NamespaceAllowlist, 2)
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
