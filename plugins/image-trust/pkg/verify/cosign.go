package verify

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
)

type CosignVerifier struct {
	runner             CommandRunner
	trustedIssuerRE    string
	trustedSubjectRE   string
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
	if issuerRE == "" {
		issuerRE = ".*"
	}
	if subjectRE == "" {
		subjectRE = ".*"
	}
	return &CosignVerifier{
		runner:           runner,
		trustedIssuerRE:  issuerRE,
		trustedSubjectRE: subjectRE,
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
		"--certificate-identity-regexp", v.trustedSubjectRE,
		"--certificate-oidc-issuer-regexp", v.trustedIssuerRE,
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

	return models.VerificationObservation{
		Mode:   v.Name(),
		Status: models.StatusVerified,
		Reason: "cosign verification succeeded",
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
