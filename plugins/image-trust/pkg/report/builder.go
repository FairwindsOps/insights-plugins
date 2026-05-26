package report

import (
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
)

const (
	defaultReason     = "verification not yet implemented"
	findingsCategory  = "ImageTrust"
	findingsSeverity  = 0.3
)

// Build creates an initial image-trust report from discovered images.
func Build(images []models.DiscoveredImage, now time.Time) models.Report {
	results := make([]models.ImageTrustResult, 0, len(images))
	findings := make([]models.Finding, 0)
	summary := models.Summary{TotalImages: len(images)}

	for _, image := range images {
		result := models.ImageTrustResult{
			Name:          image.Name,
			ID:            image.ID,
			PullRef:       image.PullRef,
			Status:        models.StatusUnknown,
			Reason:        defaultReason,
			Allowlisted:   false,
			Owners:        image.Owners,
			Signer:        models.SignerDetails{},
			LastCheckedAt: now.UTC(),
		}
		results = append(results, result)
		summary.Unknown++

		for _, owner := range image.Owners {
			findings = append(findings, models.Finding{
				ResourceNamespace: owner.Namespace,
				ResourceKind:      owner.Kind,
				ResourceName:      owner.Name,
				Title:             "Container image trust could not be verified",
				Description:       "The image-trust plugin has discovered this image, but verification is not yet implemented in the current scaffold.",
				Remediation:       "Complete the image-trust verification phase and rerun the report to determine whether this image is signed and trusted.",
				Severity:          findingsSeverity,
				Category:          findingsCategory,
			})
		}
	}

	return models.Report{
		Images:   results,
		Summary:  summary,
		Findings: findings,
	}
}
