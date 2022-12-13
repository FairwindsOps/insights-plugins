package ci

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/commands"
	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/models"
)

func (ci *CIScan) GetPolarisReport() (models.ReportInfo, error) {
	report := models.ReportInfo{
		Report:   "polaris",
		Filename: "polaris.json",
	}
	polarisVersion, err := commands.Exec("polaris", "version")
	if err != nil {
		return report, fmt.Errorf("unable to get polaris version: %v", err)
	}
	report.Version = strings.Split(polarisVersion, ":")[1]
	// Scan with Polaris
	output, err := commands.ExecWithMessage(exec.Command("polaris", "audit", "--audit-path", ci.configFolder, "--output-file", filepath.Join(ci.config.Options.TempFolder, report.Filename)), "Audit with Polaris")
	if err != nil {
		return report, fmt.Errorf("%v: %s", err, output)
	}
	return report, nil
}

func (ci *CIScan) PolarisEnabled() bool {
	return *ci.config.Reports.Polaris.Enabled
}
