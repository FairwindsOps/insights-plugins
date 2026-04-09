package ci

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/models"
	"github.com/jstemmer/go-junit-report/v2/junit"
	"github.com/sirupsen/logrus"
)

const junitClassname = "fairwinds-insights-ci"

// SaveJUnitFile will save the
func (ci *CIScan) SaveJUnitFile(results models.ScanResults) error {
	suite := junit.Testsuite{
		Name: "CI scan",
		ID:   0,
		Time: "0.000",
	}

	for _, actionItem := range results.NewActionItems {
		suite.AddTestcase(junit.Testcase{
			Name:      actionItem.GetReadableTitle(),
			Classname: junitClassname,
			Failure: &junit.Result{
				Message: actionItem.Remediation,
				Data:    fmt.Sprintf("File: %s\nDescription: %s", actionItem.Notes, actionItem.Description),
			},
		})
	}

	for _, actionItem := range results.FixedActionItems {
		suite.AddTestcase(junit.Testcase{
			Name:      actionItem.GetReadableTitle(),
			Classname: junitClassname,
		})
	}

	var suites junit.Testsuites
	suites.AddSuite(suite)

	jUnitOutputFile := filepath.Join(ci.baseFolder, ci.config.Options.JUnitOutput)
	err := os.MkdirAll(filepath.Dir(jUnitOutputFile), os.ModePerm)
	if err != nil {
		return fmt.Errorf("could not create dir: %v", err)
	}

	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	if err := suites.WriteXML(&buf); err != nil {
		return err
	}
	err = os.WriteFile(jUnitOutputFile, buf.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("could not save file: %v", err)
	}

	logrus.Info("JUnit results file saved at ", jUnitOutputFile)

	return nil
}

func (ci *CIScan) JUnitEnabled() bool {
	return ci.config.Options.JUnitOutput != ""
}
