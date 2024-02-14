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
	"github.com/hashicorp/go-multierror"
	"github.com/sirupsen/logrus"
)

const CustomCheckRuleID = "custom-tfsec"

func (ci *CIScan) TerraformEnabled() bool {
	return *ci.config.Reports.TFSec.Enabled
}

func (ci *CIScan) ProcessTerraformPaths() (report *models.ReportInfo, errs error) {
	logrus.Infof("processing %d Terraform paths", len(ci.config.Terraform.Paths))
	if len(ci.config.Terraform.Paths) == 0 {
		return nil, nil
	}
	allErrs := new(multierror.Error)
	var reportProperties models.TFSecReportProperties
	TFSecVersion, err := commands.Exec("tfsec", "-v")
	if err != nil {
		return nil, fmt.Errorf("cannot get the version of tfsec: %v: %s", err, TFSecVersion)
	}
	TFSecVersion = strings.TrimPrefix(TFSecVersion, "v")
	for _, terraformPath := range ci.config.Terraform.Paths {
		results, err := ci.ProcessTerraformPath(terraformPath)
		if err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("unable to process path %q: %v", terraformPath, err))
		}
		if len(results) > 0 {
			reportProperties.Items = append(reportProperties.Items, results...)
		}
	}
	var allErrsCombined error = nil
	if len(allErrs.Errors) > 0 { // keep the multierror from becoming individual action items
		allErrsCombined = fmt.Errorf("%v", allErrs.Error()) // return string representation from the multierror
	}
	if len(reportProperties.Items) == 0 {
		logrus.Infof("there were no tfsec findings after processing %d paths\n", len(ci.config.Terraform.Paths))
		return nil, allErrsCombined
	}
	file, err := json.MarshalIndent(reportProperties, "", " ")
	if err != nil {
		return nil, fmt.Errorf("while encoding report output: %w", err)
	}
	report = &models.ReportInfo{
		Report:   "tfsec",
		Version:  TFSecVersion,
		Filename: "tfsec.json",
	}
	err = os.WriteFile(filepath.Join(ci.config.Options.TempFolder, report.Filename), file, 0644)
	if err != nil {
		return nil, fmt.Errorf("while writing report output: %w", err)
	}
	return report, allErrsCombined
}

func (ci *CIScan) ProcessTerraformPath(terraformPath string) ([]models.TFSecResult, error) {
	terraformPathAsFileName := strings.ReplaceAll(strings.TrimPrefix(terraformPath, ci.repoBaseFolder), "/", "_")
	outputFile := filepath.Join(ci.config.Options.TempFolder, fmt.Sprintf("tfsec-output-%s", terraformPathAsFileName))
	customChecks := ci.config.Reports.TFSec.CustomChecksFilePath != nil && *ci.config.Reports.TFSec.CustomChecksFilePath != ""
	configFile := ""
	configFilePath := ""
	if customChecks {
		logrus.Info("Adding check validation")
		configFile = "--config-file"
		configFilePath = ci.repoBaseFolder + *ci.config.Reports.TFSec.CustomChecksFilePath
	}
	// The -s avoids tfsec exiting with an error value for scan warnings.
	output, err := commands.ExecWithMessage(exec.Command("tfsec", configFile, configFilePath, "-s", "-f", "json", "-O", outputFile, ci.repoBaseFolder+"/"+terraformPath), "scanning Terraform in "+terraformPath)
	if err != nil {
		return nil, fmt.Errorf("%v: %s", err, output)
	}
	var reportProperties models.TFSecReportProperties
	data, err := os.ReadFile(outputFile)
	if err != nil {
		logrus.Errorf("Error reading tfsec output from %s: %v", outputFile, err)
		return nil, fmt.Errorf("while reading output from %s: %w", outputFile, err)
	}
	err = json.Unmarshal(data, &reportProperties)
	if err != nil {
		logrus.Errorf("Error decoding tfsec output from %s: %v", outputFile, err)
		return nil, fmt.Errorf("while decoding output from %s: %w", outputFile, err)
	}
	logrus.Infof("%d tfsec results for path %s", len(reportProperties.Items), terraformPath)
	logrus.Debugf("Removing the base repository path %q from the file name of each tfsec result", ci.repoBaseFolder)
	items := []models.TFSecResult{}
	for _, item := range reportProperties.Items {
		newFileName := item.Location.FileName
		if strings.HasPrefix(item.Location.FileName, "terraform-aws-modules/") {
			logrus.Debugf("preppending %q to filename %q because it refers to a Terraform module", terraformPath, newFileName)
			newFileName = filepath.Join(terraformPath, newFileName)
		}
		newFileName = strings.TrimPrefix(newFileName, ci.repoBaseFolder+"/") // trim base folder as-is
		absRepoBaseFolder, err := filepath.Abs(ci.repoBaseFolder)            // Also attempt to trim the absolute version of the same path
		if err != nil {
			logrus.Warnf("tfsec result filenames will retain the repository base folder of %q because it was unable to be trimmed as an absolute path: %v", ci.repoBaseFolder, err)
		} else {
			newFileName = strings.TrimPrefix(newFileName, absRepoBaseFolder+"/")
		}
		logrus.Debugf("updating filename %q to be relative to the repository: %q", item.Location.FileName, newFileName)
		item.Location.FileName = newFileName
		if len(item.RuleID) == 0 {
			item.RuleID = CustomCheckRuleID
		}
		items = append(items, item)
	}
	reportProperties.Items = items
	logrus.Debugf("tfsec output for %s: %#v", terraformPath, reportProperties)
	return reportProperties.Items, nil
}
