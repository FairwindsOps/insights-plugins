// This script minimizes the size of a trivy report by factoring out vulnerability details,
// so that details for common vulnerabilities are not duplicated for each instance of that
// vulnerability.
package main

import (
	"time"

	"github.com/thoas/go-funk"
)

type MinimizedReport struct {
	Images          []ImageDetailsWithRefs
	Vulnerabilities map[string]VulnerabilityDetails
}

type ImageDetailsWithRefs struct {
	ID        string
	Name      string
	OwnerName string
	OwnerKind string
	Namespace string
	LastScan  *time.Time
	Report    []VulnerabilityRefList
}

type VulnerabilityRefList struct {
	Target          string
	Vulnerabilities []VulnerabilityInstance
}

type VulnerabilityDetails struct {
	Title           string
	Description     string
	Severity        string
	VulnerabilityID string
	References      []string
}

type VulnerabilityInstance struct {
	InstalledVersion string
	PkgName          string
	VulnerabilityID  string
}

func minimize(images []ImageReport, lastReport MinimizedReport) MinimizedReport {
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
		imageDetailsWithRefs := ImageDetailsWithRefs{
			ID:        imageDetails.ID,
			Name:      imageDetails.Name,
			OwnerName: imageDetails.OwnerName,
			OwnerKind: imageDetails.OwnerKind,
			Namespace: imageDetails.Namespace,
			Report:    []VulnerabilityRefList{},
			LastScan:  &timestamp,
		}
		for _, vulnList := range imageDetails.Report {
			vulnRefList := VulnerabilityRefList{
				Target: vulnList.Target,
			}
			for _, vuln := range vulnList.Vulnerabilities {
				outputReport.Vulnerabilities[vuln.VulnerabilityID] = VulnerabilityDetails{
					Title:           vuln.Title,
					Description:     vuln.Description,
					References:      vuln.References,
					Severity:        vuln.Severity,
					VulnerabilityID: vuln.VulnerabilityID,
				}
				vulnerabilityExists[vuln.VulnerabilityID] = true
				vulnRefList.Vulnerabilities = append(vulnRefList.Vulnerabilities, VulnerabilityInstance{
					InstalledVersion: vuln.InstalledVersion,
					PkgName:          vuln.PkgName,
					VulnerabilityID:  vuln.VulnerabilityID,
				})
			}
			imageDetailsWithRefs.Report = append(imageDetailsWithRefs.Report, vulnRefList)
		}
		found := funk.Find(outputReport.Images, func(image ImageDetailsWithRefs) bool {
			return image.Namespace == imageDetailsWithRefs.Namespace && image.OwnerKind == imageDetailsWithRefs.OwnerKind && image.OwnerName == imageDetailsWithRefs.OwnerName
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
