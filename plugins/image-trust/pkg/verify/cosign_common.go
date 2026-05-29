package verify

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
)

type cosignVerificationRecord struct {
	Optional map[string]any `json:"optional"`
}

func extractCosignSigners(stdout string) ([]models.SignerDetails, error) {
	if strings.TrimSpace(stdout) == "" {
		return nil, nil
	}
	var records []cosignVerificationRecord
	if err := json.Unmarshal([]byte(stdout), &records); err != nil {
		return nil, err
	}

	signers := make([]models.SignerDetails, 0, len(records))
	for _, record := range records {
		signer := models.SignerDetails{
			Issuer:  optionalString(record.Optional, "Issuer"),
			Subject: optionalString(record.Optional, "Subject"),
			KeyRef:  firstNonEmpty(optionalString(record.Optional, "keyid"), optionalString(record.Optional, "KeyID"), optionalString(record.Optional, "KeyRef")),
		}
		if signer.Issuer == "" && signer.Subject == "" && signer.KeyRef == "" {
			continue
		}
		signers = append(signers, signer)
	}

	return signers, nil
}

func optionalString(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, ok := values[key]
	if !ok {
		return ""
	}
	str, ok := value.(string)
	if !ok {
		return ""
	}
	return str
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func buildAlternationRegex(exacts, regexes []string) (string, error) {
	patterns := make([]string, 0, len(exacts)+len(regexes))
	for _, exact := range exacts {
		if strings.TrimSpace(exact) == "" {
			continue
		}
		patterns = append(patterns, "^"+regexp.QuoteMeta(exact)+"$")
	}
	for _, pattern := range regexes {
		if strings.TrimSpace(pattern) == "" {
			continue
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return "", err
		}
		patterns = append(patterns, "(?:"+pattern+")")
	}
	if len(patterns) == 0 {
		return "", nil
	}
	return strings.Join(patterns, "|"), nil
}
