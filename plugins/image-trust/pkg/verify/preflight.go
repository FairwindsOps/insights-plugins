package verify

import (
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/registry"
)

// Preflight checks whether verification can run for an image before invoking cosign.
func Preflight(image models.DiscoveredImage, creds registry.Credentials) (models.VerificationObservation, bool) {
	ref := creds.VerificationReference(image.VerificationReference())
	if ref != "" {
		return models.VerificationObservation{}, false
	}
	if image.DigestResolveError != "" {
		return models.VerificationObservation{
			Status: models.StatusVerificationError,
			Reason: image.DigestResolveError,
		}, true
	}
	return models.VerificationObservation{
		Status: models.StatusUnknown,
		Reason: "image could not be resolved to an immutable digest reference",
	}, true
}
