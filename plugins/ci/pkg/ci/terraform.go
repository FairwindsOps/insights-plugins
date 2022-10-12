package ci

import (
	"encoding/json"
	"fmt"
	"os"
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
		return models.ReportInfo{}, fmt.Errorf("cannot get the version of tfsec: %w", err)
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
	err = os.WriteFile(filepath.Join(ci.config.Options.TempFolder, report.Filename), file, 0644)
	if err != nil {
		return report, fmt.Errorf("while writing report output: %w", err)
	}
	return report, nil
}

func (ci *CIScan) ProcessTerraformPath(terraformPath string) ([]models.TFSecResult, error) {
	logrus.Infof("processing terraform path %s", terraformPath)
	terraformPathAsFileName := strings.ReplaceAll(strings.TrimPrefix(terraformPath, ci.repoBaseFolder), "/", "_")
	outputFile := filepath.Join(ci.config.Options.TempFolder, fmt.Sprintf("tfsec-output-%s", terraformPathAsFileName))
	logrus.Debugf("running tfsec and outputting to %s", outputFile)
	// The -s avoids tfsec exiting with an error value for scan warnings.
	_, err := commands.ExecWithMessage(exec.Command("tfsec", "-s", "-f", "json", "-O", outputFile, filepath.Join(ci.repoBaseFolder, terraformPath)), "scanning Terraform in "+terraformPath)
	if err != nil {
		return nil, err
	}
	var output models.TFSecReportProperties
	data, err := os.ReadFile(outputFile)
	if err != nil {
		logrus.Errorf("Error reading tfsec output from %s: %v", outputFile, err)
		return nil, fmt.Errorf("while reading output from %s: %w", outputFile, err)
	}
	err = json.Unmarshal(data, &output)
	if err != nil {
		logrus.Errorf("Error decoding tfsec output from %s: %v", outputFile, err)
		return nil, fmt.Errorf("while decoding output from %s: %w", outputFile, err)
	}
	logrus.Infof("%d tfsec results for path %s", len(output.Items), terraformPath)
	logrus.Debugf("Removing the base repository path %q and prepending the repository name %q to the Location.FileName of each tfsec result", ci.repoBaseFolder, ci.config.Options.RepositoryName)
	for i, _ := range output.Items {
		newFileName := output.Items[i].Location.FileName
		newFileName = strings.TrimPrefix(newFileName, ci.repoBaseFolder) // trim base folder as-is
		absRepoBaseFolder, err := filepath.Abs(ci.repoBaseFolder)        // Also attempt to trim the absolute version of the same path
		if err != nil {
			logrus.Warnf("tfsec result filenames will retain the repository base folder of %q because it was unable to be trimmed as an absolute path: %v", ci.repoBaseFolder, err)
		} else {
			newFileName = strings.TrimPrefix(newFileName, absRepoBaseFolder+"/")
		}

		newFileName = fmt.Sprintf("%s:%s", ci.config.Options.RepositoryName, newFileName)
		output.Items[i].Location.FileName = newFileName
	}
	logrus.Debugf("tfsec output for %s: %#v", terraformPath, output)
	return output.Items, nil
}
