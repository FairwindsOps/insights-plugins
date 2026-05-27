package verify

import (
	"strings"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
)

func classifyCosignFailure(message string) (models.Status, string) {
	normalized := strings.ToLower(strings.TrimSpace(message))
	reason := strings.TrimSpace(message)
	if reason == "" {
		reason = "cosign verification failed"
	}

	switch {
	case normalized == "":
		return models.StatusVerificationError, "cosign verification failed"
	case strings.Contains(normalized, "no matching signatures"),
		strings.Contains(normalized, "no signatures found"),
		strings.Contains(normalized, "not found in transparency log") && strings.Contains(normalized, "signature"):
		return models.StatusUnsigned, reason
	case strings.Contains(normalized, "unauthorized"),
		strings.Contains(normalized, "authentication required"),
		strings.Contains(normalized, "401"),
		strings.Contains(normalized, "403"),
		strings.Contains(normalized, "denied"),
		strings.Contains(normalized, "forbidden"):
		return models.StatusVerificationError, reason
	case strings.Contains(normalized, "certificate identity"),
		strings.Contains(normalized, "oidc issuer"),
		strings.Contains(normalized, "expected identities"),
		strings.Contains(normalized, "subject"),
		strings.Contains(normalized, "issuer"):
		return models.StatusSignedUntrusted, reason
	default:
		return models.StatusVerificationError, reason
	}
}
