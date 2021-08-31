package ci

import (
	"encoding/json"
	"os/exec"
	"strings"

	"github.com/fairwindsops/insights-plugins/trivy/pkg/image"
	trivymodels "github.com/fairwindsops/insights-plugins/trivy/pkg/models"
	"github.com/sirupsen/logrus"

	"github.com/fairwindsops/insights-plugins/ci/pkg/models"
	"github.com/fairwindsops/insights-plugins/ci/pkg/util"
)

// ScanImagesWithTrivy scans the images and returns a Trivy report ready to send to Insights.
func ScanImagesWithTrivy(images []trivymodels.Image, configurationObject models.Configuration) ([]byte, string, error) {
	err := util.RunCommand(exec.Command("trivy", "--download-db-only"), "downloading trivy database")
	if err != nil {
		return nil, "", err
	}
	reportByRef := map[string][]trivymodels.VulnerabilityList{}
	for _, currentImage := range images {
		_, ok := reportByRef[currentImage.PullRef]
		if ok {
			continue
		}
		logrus.Infof("Scanning %s from file %s", currentImage.Name, currentImage.PullRef)
		results, err := image.ScanImageFile(configurationObject.Images.FolderName+currentImage.PullRef, currentImage.PullRef, configurationObject.Options.TempFolder)
		if err != nil {
			return nil, "", err
		}
		logrus.Infof("results: %#v", results)
		reportByRef[currentImage.PullRef] = results
	}

	allReports := image.ConvertTrivyResultsToImageReport(images, reportByRef)
	// Collate results
	results := image.Minimize(allReports, trivymodels.MinimizedReport{Images: make([]trivymodels.ImageDetailsWithRefs, 0), Vulnerabilities: map[string]trivymodels.VulnerabilityDetails{}})
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
