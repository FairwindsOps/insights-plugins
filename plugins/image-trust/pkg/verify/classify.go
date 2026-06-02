package verify

import (
	"strconv"
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
		strings.Contains(normalized, "no matching attestations"),
		strings.Contains(normalized, "no attestations found"),
		strings.Contains(normalized, "not found in transparency log") && strings.Contains(normalized, "signature"):
		return models.StatusUnsigned, reason
	case strings.Contains(normalized, "unauthorized"),
		strings.Contains(normalized, "authentication required"),
		containsHTTPStatus(normalized, 401),
		containsHTTPStatus(normalized, 403),
		strings.Contains(normalized, "access denied"),
		strings.Contains(normalized, "denied: access forbidden"),
		strings.Contains(normalized, "forbidden"):
		return models.StatusVerificationError, reason
	case strings.Contains(normalized, "certificate identity"),
		strings.Contains(normalized, "certificate oidc issuer"),
		strings.Contains(normalized, "oidc issuer did not match"),
		strings.Contains(normalized, "expected identities"):
		return models.StatusSignedUntrusted, reason
	default:
		return models.StatusVerificationError, reason
	}
}

func containsHTTPStatus(message string, code int) bool {
	codeStr := strconv.Itoa(code)
	return strings.Contains(message, " "+codeStr) ||
		strings.Contains(message, ":"+codeStr) ||
		strings.Contains(message, "status "+codeStr) ||
		strings.Contains(message, "status code: "+codeStr) ||
		strings.Contains(message, "http "+codeStr)
}

// IsTransientFailure reports whether a verification_error reason is worth retrying.
func IsTransientFailure(reason string) bool {
	normalized := strings.ToLower(strings.TrimSpace(reason))
	switch {
	case strings.Contains(normalized, "timeout"),
		strings.Contains(normalized, "temporary failure"),
		strings.Contains(normalized, "connection refused"),
		strings.Contains(normalized, "connection reset"),
		strings.Contains(normalized, "i/o timeout"),
		strings.Contains(normalized, "tls handshake timeout"),
		strings.Contains(normalized, "no such host"),
		containsHTTPStatus(normalized, 429),
		containsHTTPStatus(normalized, 502),
		containsHTTPStatus(normalized, 503),
		containsHTTPStatus(normalized, 504),
		strings.Contains(normalized, "too many requests"),
		strings.Contains(normalized, "service unavailable"):
		return true
	default:
		return false
	}
}
