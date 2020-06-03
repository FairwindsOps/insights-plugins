package ci

import (
	"encoding/json"
	"os/exec"
	"strings"

	"github.com/fairwindsops/insights-plugins/trivy/pkg/image"
	"github.com/fairwindsops/insights-plugins/trivy/pkg/models"
	"github.com/fairwindsops/insights-plugins/trivy/pkg/util"
)

// ScanImagesWithTrivy scans the images and returns a Trivy report ready to send to Insights.
func ScanImagesWithTrivy(images []models.Image, configurationObject Configuration) ([]byte, string, error) {
	err := util.RunCommand(exec.Command("trivy", "--download-db-only"), "downloading trivy database")
	if err != nil {
		return nil, "", err
	}
	reportByRef := map[string][]models.VulnerabilityList{}
	for _, currentImage := range images {
		results, err := image.ScanImageFile(configurationObject.Images.FolderName+currentImage.PullRef, currentImage.PullRef, configurationObject.Options.TempFolder)
		if err != nil {
			return nil, "", err
		}
		reportByRef[currentImage.PullRef] = results
	}

	allReports := image.ConvertTrivyResultsToImageReport(images, reportByRef)
	// Collate results
	results := image.Minimize(allReports, models.MinimizedReport{Images: make([]models.ImageDetailsWithRefs, 0), Vulnerabilities: map[string]models.VulnerabilityDetails{}})
	trivyResults, err := json.Marshal(results)
	if err != nil {
		return nil, "", err
	}

	trivyVersion, err := GetResultsFromCommand("trivy", "--version")
	if err != nil {
		return nil, "", err
	}
	trivyVersion = strings.Split(strings.Split(trivyVersion, "\n")[0], " ")[1]
	return trivyResults, trivyVersion, nil
}
