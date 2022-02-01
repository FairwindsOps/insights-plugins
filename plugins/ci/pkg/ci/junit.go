package ci

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/fairwindsops/insights-plugins/ci/pkg/models"
	"github.com/jstemmer/go-junit-report/formatter"
)

// SaveJUnitFile will save the
func (ci *CI) SaveJUnitFile(results models.ScanResults) error {
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

	err := os.MkdirAll(filepath.Dir(ci.config.Options.JUnitOutput), 0644)
	if err != nil {
		return err
	}

	xmlBytes, err := xml.MarshalIndent(testSuites, "", "\t")
	if err != nil {
		return err
	}
	xmlBytes = append([]byte(xml.Header), xmlBytes...)
	err = ioutil.WriteFile(ci.config.Options.JUnitOutput, xmlBytes, 0644)
	if err != nil {
		return err
	}

	return nil
}

func (ci *CI) JUnitEnabled() bool {
	return ci.config.Options.JUnitOutput != ""
}
