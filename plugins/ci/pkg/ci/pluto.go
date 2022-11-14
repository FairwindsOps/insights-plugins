package ci

import (
	"os"
	"path/filepath"

	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/commands"
	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/models"
)

func (ci *CIScan) GetPlutoReport() (models.ReportInfo, error) {
	report := models.ReportInfo{
		Report:   "pluto",
		Filename: "pluto.json",
	}
	// Scan with Pluto
	plutoResults, err := commands.Exec("pluto", "detect-files", "-d", ci.configFolder, "-o", "json", "--ignore-deprecations", "--ignore-removals")
	if err != nil {
		return report, err
	}
	err = os.WriteFile(filepath.Join(ci.config.Options.TempFolder, report.Filename), []byte(plutoResults), 0644)
	if err != nil {
		return report, err
	}
	report.Version = os.Getenv("plutoVersion")
	return report, nil
}

func (ci *CIScan) PlutoEnabled() bool {
	return *ci.config.Reports.Pluto.Enabled
}
