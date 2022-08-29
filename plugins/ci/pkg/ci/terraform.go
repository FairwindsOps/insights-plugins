package ci

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/commands"
	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/models"
	"github.com/sirupsen/logrus"
)

func (ci *CIScan) ProcessTerraformPaths() (models.ReportInfo, error) {
	logrus.Infof("processing %d Terraform paths", len(ci.config.Terraform.Paths))
	var reportProperties models.TFSecReportProperties
	for _, terraformPath := range ci.config.Terraform.Paths {
		results, err := ci.ProcessTerraformPath(filepath.Join(ci.repoBaseFolder, terraformPath))
		if err != nil {
			return models.ReportInfo{}, err
		}
		if len(results) > 0 {
			reportProperties.Items = append(reportProperties.Items, results...)
		}
	}
	TFSecVersion, err := commands.Exec("tfsec", "-v")
	if err != nil {
		return models.ReportInfo{}, fmt.Errorf("cannot get tfsec version: %w", err)
	}
	TFSecVersion = strings.TrimPrefix(TFSecVersion, "v")
	report := models.ReportInfo{
		Report:   "tfsec",
		Version:  TFSecVersion,
		Filename: "tfsec.json",
	}
	file, err := json.MarshalIndent(reportProperties, "", " ")
	if err != nil {
		return report, fmt.Errorf("while encoding report output: %w", err)
	}
	err = ioutil.WriteFile(report.Filename, file, 0644)
	if err != nil {
		return report, fmt.Errorf("while writing report output: %w", err)
	}
	return report, nil
}

func (ci *CIScan) ProcessTerraformPath(terraformPath string) ([]models.TFSecResult, error) {
	logrus.Infof("processing terraform path %s", terraformPath)
	terraformPathAsFileName := strings.ReplaceAll(strings.TrimPrefix(terraformPath, ci.repoBaseFolder), "/", "")
	outputFile := filepath.Join(ci.config.Options.TempFolder, fmt.Sprintf("tfsec-output-%s", terraformPathAsFileName))
	logrus.Infof("running tfsec and outputting to %s", outputFile)
	_, err := commands.ExecWithMessage(exec.Command("tfsec", "-f", "json", "-O", outputFile, filepath.Join(ci.repoBaseFolder, terraformPath)), "scanning Terraform in "+terraformPath)
	if err != nil {
		return nil, err
	}
	var output models.TFSecReportProperties
	data, err := ioutil.ReadFile(outputFile)
	if err != nil {
		logrus.Errorf("Error reading tfsec output from %s: %v", outputFile, err)
		return nil, fmt.Errorf("while reading output from %s: %w", err)
	}
	err = json.Unmarshal(data, &output)
	if err != nil {
		logrus.Errorf("Error decoding tfsec output from %s: %v", outputFile, err)
		return nil, fmt.Errorf("while decoding output from %s: %w", err)
	}
	logrus.Infof("%d tfsec results for path %s", len(output.Items), terraformPath)
	logrus.Debugf("tfsec output for %s: %#v", terraformPath, output)
	return output.Items, nil
}
