package main

import (
	"github.com/fairwindsops/insights-plugins/trivy/pkg/image"
	"github.com/fairwindsops/insights-plugins/trivy/pkg/models"
	"github.com/thoas/go-funk"
)

func main() {
	// Parse out config?
	// Scan YAML, find all images/kind/etc
	// Scan Images with Trivy
	reportByRef := funk.Map(images, func(image models.Image) (string, []models.VulnerabilityList) {
		results, _ := util.ScanImageFile(image.PullRef)
		return image.PullRef, results
	})
	allReports := image.ConvertTrivyResultsToImageReport(images, reportByRef)
	// Collate results
	finalReport := image.Minimize(allReports, models.MinimizedReport{Images: make([]models.ImageDetailsWithRefs, 0), Vulnerabilities: map[string]models.VulnerabilityDetails{}})

	// Scan with Polaris
	// Send Results up
}
