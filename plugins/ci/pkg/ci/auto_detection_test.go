package ci

import (
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/models"
	"github.com/stretchr/testify/assert"
)

// RunCommand runs a command and prints errors to Stderr
func TestAutoDetection(t *testing.T) {

	cfg, err := ConfigFileAutoDetection("./testdata/repo1")
	assert.NoError(t, err)

	expected := models.Configuration{
		Manifests: models.ManifestConfig{
			YamlPaths: []string{
				"failing.yaml",
				"passing.yaml",
				"yml/failing_2.yml",
				"yml/json/failing_3.json",
			},
			Helm: []models.HelmConfig{
				{
					Name:       "helm-release-pruner-json",
					Path:       "chart_json",
					ValuesFile: "chart_json/values.json",
				},
				{
					Name:       "helm-release-pruner-yaml",
					Path:       "chart_yaml",
					ValuesFile: "chart_yaml/values.yaml",
				},
				{
					Name:       "helm-release-pruner-yml",
					Path:       "chart_yml",
					ValuesFile: "chart_yml/values.yml",
				},
			},
		},
	}
	assert.Equal(t, expected, *cfg)
}
