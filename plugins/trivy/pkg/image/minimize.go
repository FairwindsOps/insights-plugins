// This script minimizes the size of a trivy report by factoring out vulnerability details,
// so that details for common vulnerabilities are not duplicated for each instance of that
// vulnerability.
package image

import (
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/models"
	v2 "github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/models/v2"
)

// Minimize compresses the format of the Trivy report to de-duplicate information about vulnerabilities.
func Minimize(images []models.ImageReport, lastReport v2.MinimizedReport) v2.MinimizedReport {
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
		var owners []v2.Resource
		for _, owner := range imageDetails.Owners {
			owners = append(owners, v2.Resource{
				Kind:      owner.Kind,
				Namespace: owner.Namespace,
				Name:      owner.Name,
				Container: owner.Container,
			})
		}
		imageDetailsWithRefs := v2.ImageDetailsWithRefs{
			ID:                 imageDetails.ID,
			Name:               imageDetails.Name,
			OSArch:             imageDetails.OSArch,
			Owners:             owners,
			Report:             []v2.VulnerabilityRefList{},
			LastScan:           &timestamp,
			RecommendationOnly: imageDetails.RecommendationOnly,
		}
		for _, vulnList := range imageDetails.Reports {
			vulnRefList := v2.VulnerabilityRefList{
				Target: vulnList.Target,
			}
			for _, vuln := range vulnList.Vulnerabilities {
				outputReport.Vulnerabilities[vuln.VulnerabilityID] = v2.VulnerabilityDetails{
					Title:           vuln.Title,
					Description:     vuln.Description,
					References:      vuln.References,
					Severity:        vuln.Severity,
					VulnerabilityID: vuln.VulnerabilityID,
				}
				vulnerabilityExists[vuln.VulnerabilityID] = true
				vulnRefList.Vulnerabilities = append(vulnRefList.Vulnerabilities, v2.VulnerabilityInstance{
					InstalledVersion: vuln.InstalledVersion,
					PkgName:          vuln.PkgName,
					VulnerabilityID:  vuln.VulnerabilityID,
					FixedVersion:     vuln.FixedVersion,
				})
			}
			imageDetailsWithRefs.Report = append(imageDetailsWithRefs.Report, vulnRefList)
		}
		outputReport.Images = append(outputReport.Images, imageDetailsWithRefs)
	}
	for vulnID := range outputReport.Vulnerabilities {
		if !vulnerabilityExists[vulnID] {
			delete(outputReport.Vulnerabilities, vulnID)
		}
	}
	return outputReport
}
