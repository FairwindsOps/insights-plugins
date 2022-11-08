package ci

import (
	"strings"
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/models"
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
	assert.Equal(t, models.GithubActions, cfg.Options.CIRunner)
}
