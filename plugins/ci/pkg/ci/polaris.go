package ci

import (
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fairwindsops/insights-plugins/ci/pkg/commands"
	"github.com/fairwindsops/insights-plugins/ci/pkg/models"
)

func (ci *CI) GetPolarisReport() (models.ReportInfo, error) {
	report := models.ReportInfo{
		Report:   "polaris",
		Filename: "polaris.json",
	}
	// Scan with Polaris
	_, err := commands.ExecWithMessage(exec.Command("polaris", "audit", "--audit-path", ci.configFolder, "--output-file", filepath.Join(ci.config.Options.TempFolder, report.Filename)), "Audit with Polaris")
	if err != nil {
		return report, err
	}
	polarisVersion, err := commands.Exec("polaris", "version")
	if err != nil {
		return report, err
	}
	report.Version = strings.Split(polarisVersion, ":")[1]
	return report, nil
}

func (ci *CI) PolarisEnabled() bool {
	return *ci.config.Reports.Polaris.Enabled
}
