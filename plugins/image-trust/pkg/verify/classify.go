package verify

import (
	"strings"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
)

func classifyCosignFailure(message string) (models.Status, string) {
	normalized := strings.ToLower(strings.TrimSpace(message))
	switch {
	case normalized == "":
		return models.StatusVerificationError, "cosign verification failed"
	case strings.Contains(normalized, "no matching signatures"):
		return models.StatusUnsigned, strings.TrimSpace(message)
	case strings.Contains(normalized, "certificate identity"),
		strings.Contains(normalized, "oidc issuer"),
		strings.Contains(normalized, "expected identities"),
		strings.Contains(normalized, "subject"),
		strings.Contains(normalized, "issuer"):
		return models.StatusSignedUntrusted, strings.TrimSpace(message)
	default:
		return models.StatusVerificationError, strings.TrimSpace(message)
	}
}
