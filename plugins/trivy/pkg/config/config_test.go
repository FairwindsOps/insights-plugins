package config

import (
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
	assert.Equal(t, 10, cfg.MaxImagesToScan)
	assert.Empty(t, cfg.ExtraFlags)
	assert.False(t, cfg.Offline)
	assert.Empty(t, cfg.ImagesToScan)
}

func TestLoadFromEnvironment(t *testing.T) {
	t.Setenv("SERVICE_ACCOUNT_ANNOTATIONS", `{"iam.gke.io/gcp-service-account":"my-gsa@my-project.iam.gserviceaccount.com","another-key":"another-value"}`)
	t.Setenv("MAX_CONCURRENT_SCANS", "99")
	t.Setenv("MAX_SCANS", "88")
	t.Setenv("IGNORE_UNFIXED", "true")
	t.Setenv("OFFLINE", "true")
	t.Setenv("NAMESPACE_BLOCKLIST", "kube-system,kube-public")
	t.Setenv("NAMESPACE_ALLOWLIST", "default,fw-insights")
	t.Setenv("IMAGES_TO_SCAN", "nginx:latest,redis:alpine")

	cfg, err := LoadFromEnvironment()
	assert.NoError(t, err)

	assert.True(t, cfg.HasGKESAAnnotation, 1)
	assert.Len(t, cfg.NamespaceBlocklist, 2)
	assert.Len(t, cfg.NamespaceAllowlist, 2)
	assert.Equal(t, 99, cfg.MaxConcurrentScans)
	assert.Equal(t, 88, cfg.MaxImagesToScan)
	assert.Equal(t, "--ignore-unfixed", cfg.ExtraFlags)
	assert.True(t, cfg.Offline)
	assert.Equal(t, []string{"nginx:latest", "redis:alpine"}, cfg.ImagesToScan)
}
