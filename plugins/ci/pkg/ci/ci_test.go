package ci

import (
	"os"
	"strings"
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/models"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
)

func TestReadConfigurationFromFile(t *testing.T) {
	configFileContent := `
options:
  organization: fairwinds-production
  baseBranch: master
  repositoryName: FairwindsOps/fairwindsops-infrastructure
  ciRunner: github-actions

manifests:
  yaml:
  - ./inventory/production/clusters/prod/resources/manifests

reports:
  trivy:
    enabled: false
  opa:
    enabled: false
  prometheus-metrics:
    enabled: true
  goldilocks:
    enabled: false

exemptions:
  - image: 602401143452.dkr.ecr.us-west-2.amazonaws.com/amazon/aws-iam-authenticator:v0.5.2-scratch
  - image: 602401143452.dkr.ecr.us-west-2.amazonaws.com/amazon/aws-iam-authenticator:v0.5.3-scratch
`
	reader := strings.NewReader(configFileContent)
	cfg, err := readConfigurationFromReader(reader)
	assert.NoError(t, err)
	assert.Equal(t, "fairwinds-production", cfg.Options.Organization)
	assert.Equal(t, "master", cfg.Options.BaseBranch)
	assert.Equal(t, "FairwindsOps/fairwindsops-infrastructure", cfg.Options.RepositoryName)
	assert.Empty(t, cfg.Options.CIRunner) // should not be read from file
	assert.True(t, *cfg.Reports.PrometheusMetrics.Enabled)
	assert.False(t, *cfg.Reports.Goldilocks.Enabled)
}

func TestUnmarshalAndOverrideConfig(t *testing.T) {
	os.Setenv("REPORTS_CONFIG", "{}")

	cfg := models.Configuration{}
	err := unmarshalAndOverrideConfig(&cfg)
	assert.NoError(t, err)
	assert.Equal(t, models.Configuration{}, cfg)

	os.Setenv("REPORTS_CONFIG", `{"autoScan": {}}`)

	cfg = models.Configuration{}
	err = unmarshalAndOverrideConfig(&cfg)
	assert.NoError(t, err)
	assert.Equal(t, models.Configuration{}, cfg)

	os.Setenv("REPORTS_CONFIG", `{"autoScan": {"polaris": {"enabledOnAutoDiscovery": true}}}`)

	cfg = models.Configuration{}
	err = unmarshalAndOverrideConfig(&cfg)
	assert.NoError(t, err)
	expected := models.Configuration{}
	expected.Reports.Polaris.Enabled = lo.ToPtr(true)
	assert.Equal(t, expected, cfg)
}

func TestHasEnvSubstitution(t *testing.T) {
	assert.True(t, hasEnvVar("161177611123.dkr.ecr.us-east-1.amazonaws.com/fairwinds-insights-api:$CI_SHA1"))
	assert.False(t, hasEnvVar("161177611123.dkr.ecr.us-east-1.amazonaws.com/fairwinds-insights-api:5541f8d19d1e0a1ae860388c8b25b737773fd6ec"))
	assert.False(t, hasEnvVar("161177611123.dkr.ecr.us-east-1.amazonaws.com/fairwinds-insights-api:11.0.0"))
}

func TestGetAllResources(t *testing.T) {
	ci := CIScan{configFolder: "testdata/walk", config: &models.Configuration{}}
	images, resources, err := ci.getAllResources()
	assert.Equal(t, err.Error(), `2 errors occurred:
	* error decoding document testdata/walk/helm-release-pruner-yaml/templates/configmap_error.yml: yaml: unmarshal errors:
  line 5: mapping key "kind" already defined at line 4
	* error decoding document testdata/walk/helm-release-pruner-yaml/templates/cronjob_error.yml: yaml: unmarshal errors:
  line 5: mapping key "kind" already defined at line 4

`)
	assert.Len(t, images, 3, "even though there are errors, we should still get the images")
	assert.Len(t, resources, 7, "even though there are errors, we should still get the resources")
}

func TestTrimSpace(t *testing.T) {
	assert.Equal(t, "hello", strings.TrimSpace("hello"))
	assert.Equal(t, "hello", strings.TrimSpace(" hello "))
	assert.Equal(t, "hello", strings.TrimSpace("\nhello\n"))
	assert.Equal(t, "hello", strings.TrimSpace("\n hello \n"))
	assert.Equal(t, "hello", strings.TrimSpace("\n hello"))
	assert.Equal(t, "hello", strings.TrimSpace("hello\n"))
	assert.Equal(t, "hello", strings.TrimSpace("\nhello"))
	assert.Equal(t, "hello", strings.TrimSpace(" \n hello \n "))
}
