package ci

import (
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/models"
	"github.com/jstemmer/go-junit-report/v2/junit"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveJUnitFile_output(t *testing.T) {
	logrus.SetOutput(io.Discard)
	t.Cleanup(func() { logrus.SetOutput(os.Stderr) })

	dir := t.TempDir()
	outRel := filepath.Join("reports", "junit.xml")
	cfg := &models.Configuration{}
	cfg.Options.JUnitOutput = outRel

	ci := &CIScan{
		baseFolder: dir,
		config:     cfg,
	}

	results := models.ScanResults{
		NewActionItems: []models.ActionItem{
			{
				Remediation: "apply the patch",
				Title:       "policy violation",
				Description: "details here",
				Notes:       "manifest.yaml",
				Resource: models.K8sResource{
					Namespace: "prod",
					Kind:      "Deployment",
					Name:      "api",
				},
			},
		},
		FixedActionItems: []models.ActionItem{
			{
				Title: "resolved check",
				Resource: models.K8sResource{
					Kind: "Service",
					Name: "cache",
				},
			},
		},
	}

	err := ci.SaveJUnitFile(results)
	require.NoError(t, err)

	path := filepath.Join(dir, outRel)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	out := string(raw)
	assert.True(t, strings.HasPrefix(out, xml.Header), "should start with XML declaration")
	assert.Contains(t, out, `<testsuites tests="2" failures="1"`)
	assert.Contains(t, out, `<testsuite name="CI scan"`)
	assert.Contains(t, out, `tests="2" failures="1" errors="0" id="0"`)
	assert.Contains(t, out, `classname="fairwinds-insights-ci"`)

	failureName := `prod/Deployment/api - policy violation`
	assert.Contains(t, out, `name="`+failureName+`"`)
	assert.Contains(t, out, `message="apply the patch"`)
	assert.Contains(t, out, "<![CDATA[File: manifest.yaml\nDescription: details here]]>")

	passName := `Service/cache - resolved check`
	assert.Contains(t, out, `name="`+passName+`"`)

	var suites junit.Testsuites
	err = xml.Unmarshal(raw, &suites)
	require.NoError(t, err, "output should unmarshal back as junit.Testsuites")
	require.Len(t, suites.Suites, 1)
	suite := suites.Suites[0]
	assert.Equal(t, "CI scan", suite.Name)
	assert.Equal(t, 2, suite.Tests)
	assert.Equal(t, 1, suite.Failures)
	require.Len(t, suite.Testcases, 2)
	assert.Equal(t, failureName, suite.Testcases[0].Name)
	require.NotNil(t, suite.Testcases[0].Failure)
	assert.Equal(t, "apply the patch", suite.Testcases[0].Failure.Message)
	assert.Equal(t, passName, suite.Testcases[1].Name)
	assert.Nil(t, suite.Testcases[1].Failure)
}

func TestCIScan_JUnitEnabled(t *testing.T) {
	ci := &CIScan{config: &models.Configuration{}}
	assert.False(t, ci.JUnitEnabled())

	ci.config.Options.JUnitOutput = "results/junit.xml"
	assert.True(t, ci.JUnitEnabled())
}
