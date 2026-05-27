package verify

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
)

type CosignVerifier struct {
	runner                CommandRunner
	trustedIssuerMatcher  *regexp.Regexp
	trustedSubjectMatcher *regexp.Regexp
}

func NewCosignVerifier(runner CommandRunner, trustedIssuers, trustedSubjects, trustedSubjectREs []string) (*CosignVerifier, error) {
	issuerRE, err := buildAlternationRegex(trustedIssuers, nil)
	if err != nil {
		return nil, fmt.Errorf("building issuer regex: %w", err)
	}
	subjectRE, err := buildAlternationRegex(trustedSubjects, trustedSubjectREs)
	if err != nil {
		return nil, fmt.Errorf("building subject regex: %w", err)
	}

	var issuerMatcher *regexp.Regexp
	if issuerRE != "" {
		issuerMatcher, err = regexp.Compile(issuerRE)
		if err != nil {
			return nil, fmt.Errorf("compiling issuer regex: %w", err)
		}
	}

	var subjectMatcher *regexp.Regexp
	if subjectRE != "" {
		subjectMatcher, err = regexp.Compile(subjectRE)
		if err != nil {
			return nil, fmt.Errorf("compiling subject regex: %w", err)
		}
	}

	return &CosignVerifier{
		runner:                runner,
		trustedIssuerMatcher:  issuerMatcher,
		trustedSubjectMatcher: subjectMatcher,
	}, nil
}

func (v *CosignVerifier) Name() models.VerificationMode {
	return models.VerificationModeCosignKeyless
}

func (v *CosignVerifier) Verify(ctx context.Context, image models.DiscoveredImage) (models.VerificationObservation, error) {
	ref := image.VerificationReference()
	if ref == "" {
		return models.VerificationObservation{
			Mode:   v.Name(),
			Status: models.StatusUnknown,
			Reason: "image could not be resolved to an immutable digest reference",
		}, nil
	}

	args := []string{
		"verify",
		"--output", "json",
		"--certificate-identity-regexp", ".*",
		"--certificate-oidc-issuer-regexp", ".*",
		ref,
	}

	stdout, stderr, err := v.runner.Run(ctx, "cosign", args...)
	if err != nil {
		if strings.Contains(err.Error(), "executable file not found") {
			return models.VerificationObservation{
				Mode:   v.Name(),
				Status: models.StatusVerificationError,
				Reason: "cosign binary not available",
			}, nil
		}
		status, reason := classifyCosignFailure(firstNonEmpty(stderr, stdout, err.Error()))
		return models.VerificationObservation{
			Mode:   v.Name(),
			Status: status,
			Reason: reason,
		}, nil
	}

	signers, err := extractCosignSigners(stdout)
	if err != nil {
		return models.VerificationObservation{
			Mode:   v.Name(),
			Status: models.StatusVerificationError,
			Reason: fmt.Sprintf("cosign verification succeeded but signer data could not be parsed: %v", err),
		}, nil
	}

	if v.hasTrustPolicy() {
		if len(signers) == 0 {
			return models.VerificationObservation{
				Mode:   v.Name(),
				Status: models.StatusVerificationError,
				Reason: "cosign verification succeeded but signer identity could not be determined",
			}, nil
		}
		for _, signer := range signers {
			if v.isTrustedSigner(signer) {
				return models.VerificationObservation{
					Mode:    v.Name(),
					Status:  models.StatusVerified,
					Reason:  "cosign verification succeeded",
					Signer:  signer,
					Signers: signers,
				}, nil
			}
		}
		return models.VerificationObservation{
			Mode:    v.Name(),
			Status:  models.StatusSignedUntrusted,
			Reason:  "signature was verified but no signer matched the configured trust policy",
			Signer:  signers[0],
			Signers: signers,
		}, nil
	}

	primarySigner := models.SignerDetails{}
	if len(signers) > 0 {
		primarySigner = signers[0]
	}

	return models.VerificationObservation{
		Mode:    v.Name(),
		Status:  models.StatusVerified,
		Reason:  "cosign verification succeeded",
		Signer:  primarySigner,
		Signers: signers,
	}, nil
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (v *CosignVerifier) hasTrustPolicy() bool {
	return v.trustedIssuerMatcher != nil || v.trustedSubjectMatcher != nil
}

func (v *CosignVerifier) isTrustedSigner(signer models.SignerDetails) bool {
	if v.trustedIssuerMatcher != nil && !v.trustedIssuerMatcher.MatchString(signer.Issuer) {
		return false
	}
	if v.trustedSubjectMatcher != nil && !v.trustedSubjectMatcher.MatchString(signer.Subject) {
		return false
	}
	return true
}

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
