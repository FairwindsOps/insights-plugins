package ci

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/models"
	"github.com/jstemmer/go-junit-report/formatter"
	"github.com/sirupsen/logrus"
)

// SaveJUnitFile will save the
func (ci *CIScan) SaveJUnitFile(results models.ScanResults) error {
	cases := make([]formatter.JUnitTestCase, 0)

	for _, actionItem := range results.NewActionItems {
		cases = append(cases, formatter.JUnitTestCase{
			Name: actionItem.GetReadableTitle(),
			Failure: &formatter.JUnitFailure{
				Message:  actionItem.Remediation,
				Contents: fmt.Sprintf("File: %s\nDescription: %s", actionItem.Notes, actionItem.Description),
			},
		})
	}

	for _, actionItem := range results.FixedActionItems {
		cases = append(cases, formatter.JUnitTestCase{
			Name: actionItem.GetReadableTitle(),
		})
	}

	testSuites := formatter.JUnitTestSuites{
		Suites: []formatter.JUnitTestSuite{
			{
				Tests:     len(results.NewActionItems) + len(results.FixedActionItems),
				TestCases: cases,
			},
		},
	}

	jUnitOutputFile := filepath.Join(ci.baseFolder, ci.config.Options.JUnitOutput)
	err := os.MkdirAll(filepath.Dir(jUnitOutputFile), os.ModePerm)
	if err != nil {
		return fmt.Errorf("could not create dir: %v", err)
	}

	xmlBytes, err := xml.MarshalIndent(testSuites, "", "\t")
	if err != nil {
		return err
	}
	xmlBytes = append([]byte(xml.Header), xmlBytes...)
	err = ioutil.WriteFile(jUnitOutputFile, xmlBytes, 0644)
	if err != nil {
		return fmt.Errorf("could not save file: %v", err)
	}

	logrus.Info("JUnit results file saved at ", jUnitOutputFile)

	return nil
}

func (ci *CIScan) JUnitEnabled() bool {
	return ci.config.Options.JUnitOutput != ""
}
