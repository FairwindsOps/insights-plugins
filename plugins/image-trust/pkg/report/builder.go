package report

import (
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
)

const (
	findingsCategory  = "ImageTrust"
	verificationErrorSeverity = 0.3
	nonCompliantSeverity      = 0.7
)

// Build creates an image-trust report from final image results.
func Build(results []models.ImageTrustResult) models.Report {
	findings := make([]models.Finding, 0)
	summary := models.Summary{TotalImages: len(results)}

	for _, result := range results {
		incrementSummary(&summary, result)
		if result.Allowlisted || result.Status == models.StatusVerified {
			continue
		}
		for _, owner := range result.Owners {
			findings = append(findings, buildFinding(owner, result))
		}
	}

	return models.Report{
		Images:   results,
		Summary:  summary,
		Findings: findings,
	}
}

func incrementSummary(summary *models.Summary, result models.ImageTrustResult) {
	switch result.Status {
	case models.StatusVerified:
		summary.Verified++
	case models.StatusUnsigned:
		summary.Unsigned++
	case models.StatusSignedUntrusted:
		summary.SignedUntrusted++
	case models.StatusVerificationError:
		summary.VerificationError++
	case models.StatusUnknown:
		summary.Unknown++
	}
	if result.Allowlisted {
		summary.Allowlisted++
	}
}

func buildFinding(owner models.Resource, result models.ImageTrustResult) models.Finding {
	title, description, remediation, severity := detailsForStatus(result)
	return models.Finding{
		ResourceNamespace: owner.Namespace,
		ResourceKind:      owner.Kind,
		ResourceName:      owner.Name,
		Title:             title,
		Description:       description,
		Remediation:       remediation,
		Severity:          severity,
		Category:          findingsCategory,
	}
}

func detailsForStatus(result models.ImageTrustResult) (title, description, remediation string, severity float64) {
	imageRef := imageReference(result)
	switch result.Status {
	case models.StatusUnsigned:
		return "Container image is unsigned",
			"The image-trust plugin did not find a matching Cosign signature for image " + imageRef + ".",
			"Sign the image in CI with Cosign keyless signing and redeploy the workload using the signed digest reference.",
			nonCompliantSeverity
	case models.StatusSignedUntrusted:
		return "Container image is signed by an untrusted signer",
			"The image-trust plugin verified a Cosign signature on image " + imageRef + ", but the signer did not match the configured trusted issuer/subject policy.",
			"Update signer trust configuration or publish the image using an approved signing identity.",
			nonCompliantSeverity
	case models.StatusVerificationError:
		return "Container image trust could not be verified",
			"The image-trust plugin encountered an operational error while verifying image " + imageRef + ": " + result.Reason,
			"Fix registry access, network connectivity, or verifier configuration and rerun the report.",
			verificationErrorSeverity
	case models.StatusUnknown:
		return "Container image trust is unknown",
			"The image-trust plugin could not resolve image " + imageRef + " to an immutable digest reference for Cosign verification.",
			"Ensure the workload resolves to a digest-backed image reference or that runtime image metadata is available.",
			verificationErrorSeverity
	default:
		return "Container image trust requires attention",
			"Image trust status for " + imageRef + " requires review.",
			"Review image trust configuration and rerun the report.",
			verificationErrorSeverity
	}
}

func imageReference(result models.ImageTrustResult) string {
	if result.ID != "" {
		return result.ID
	}
	if result.Name != "" {
		return result.Name
	}
	return "(unknown image)"
}
