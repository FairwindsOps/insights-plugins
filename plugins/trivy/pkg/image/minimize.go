// This script minimizes the size of a trivy report by factoring out vulnerability details,
// so that details for common vulnerabilities are not duplicated for each instance of that
// vulnerability.
package image

import (
	"time"

	"github.com/fairwindsops/insights-plugins/trivy/pkg/models"
	"github.com/thoas/go-funk"
)

// Minimize compresses the format of the Trivy report to de-duplicate information about vulnerabilities.
func Minimize(images []models.ImageReport, lastReport models.MinimizedReport) models.MinimizedReport {
	outputReport := lastReport
	timestamp := time.Now()
	vulnerabilityExists := map[string]bool{}
	for _, image := range outputReport.Images {
		for _, vulnList := range image.Report {
			for _, vulnerability := range vulnList.Vulnerabilities {
				vulnerabilityExists[vulnerability.VulnerabilityID] = true
			}
		}
	}
	for _, imageDetails := range images {
		imageDetailsWithRefs := models.ImageDetailsWithRefs{
			ID:                 imageDetails.ID,
			Name:               imageDetails.Name,
			OwnerName:          imageDetails.OwnerName,
			OwnerKind:          imageDetails.OwnerKind,
			OwnerContainer:     imageDetails.OwnerContainer,
			Namespace:          imageDetails.Namespace,
			Report:             []models.VulnerabilityRefList{},
			LastScan:           &timestamp,
			RecommendationOnly: imageDetails.RecommendationOnly,
		}
		for _, vulnList := range imageDetails.Report {
			vulnRefList := models.VulnerabilityRefList{
				Target: vulnList.Target,
			}
			for _, vuln := range vulnList.Vulnerabilities {
				outputReport.Vulnerabilities[vuln.VulnerabilityID] = models.VulnerabilityDetails{
					Title:           vuln.Title,
					Description:     vuln.Description,
					References:      vuln.References,
					Severity:        vuln.Severity,
					VulnerabilityID: vuln.VulnerabilityID,
				}
				vulnerabilityExists[vuln.VulnerabilityID] = true
				vulnRefList.Vulnerabilities = append(vulnRefList.Vulnerabilities, models.VulnerabilityInstance{
					InstalledVersion: vuln.InstalledVersion,
					PkgName:          vuln.PkgName,
					VulnerabilityID:  vuln.VulnerabilityID,
					FixedVersion:     vuln.FixedVersion,
				})
			}
			imageDetailsWithRefs.Report = append(imageDetailsWithRefs.Report, vulnRefList)
		}
		found := funk.Find(outputReport.Images, func(image models.ImageDetailsWithRefs) bool {
			return image.Namespace == imageDetailsWithRefs.Namespace && image.OwnerKind == imageDetailsWithRefs.OwnerKind && image.OwnerName == imageDetailsWithRefs.OwnerName && image.Name == imageDetailsWithRefs.Name
		})
		if found == nil {
			outputReport.Images = append(outputReport.Images, imageDetailsWithRefs)
		}
	}
	for vulnID := range outputReport.Vulnerabilities {
		if !vulnerabilityExists[vulnID] {
			delete(outputReport.Vulnerabilities, vulnID)
		}
	}
	return outputReport
}
