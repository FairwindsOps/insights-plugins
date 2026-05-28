package verify

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/registry"
)

// CosignAttestationVerifier verifies in-toto attestations with keyless Cosign.
type CosignAttestationVerifier struct {
	runner                CommandRunner
	registryCreds         registry.Credentials
	attestationTypes      []string
	trustedIssuerMatcher  *regexp.Regexp
	trustedSubjectMatcher *regexp.Regexp
}

// NewCosignAttestationVerifier creates a keyless attestation verifier.
func NewCosignAttestationVerifier(
	runner CommandRunner,
	registryCreds registry.Credentials,
	attestationTypes []string,
	trustedIssuers, trustedSubjects, trustedSubjectREs []string,
) (*CosignAttestationVerifier, error) {
	if len(attestationTypes) == 0 {
		return nil, fmt.Errorf("at least one attestation type is required")
	}
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

	return &CosignAttestationVerifier{
		runner:                runner,
		registryCreds:         registryCreds,
		attestationTypes:      append([]string(nil), attestationTypes...),
		trustedIssuerMatcher:  issuerMatcher,
		trustedSubjectMatcher: subjectMatcher,
	}, nil
}

func (v *CosignAttestationVerifier) Name() models.VerificationMode {
	return models.VerificationModeCosignAttestationKeyless
}

func (v *CosignAttestationVerifier) Verify(ctx context.Context, image models.DiscoveredImage) (models.VerificationObservation, error) {
	ref := v.registryCreds.VerificationReference(image.VerificationReference())
	if ref == "" {
		return models.VerificationObservation{
			Mode:   v.Name(),
			Status: models.StatusUnknown,
			Reason: "image could not be resolved to an immutable digest reference",
		}, nil
	}

	if !v.hasTrustPolicy() {
		return models.VerificationObservation{
			Mode:   v.Name(),
			Status: models.StatusVerificationError,
			Reason: "trusted issuer or subject policy is not configured",
		}, nil
	}

	var attempts []models.VerificationObservation
	for _, attestationType := range v.attestationTypes {
		observation, err := v.verifyType(ctx, ref, attestationType)
		if err != nil {
			return models.VerificationObservation{}, err
		}
		if observation.Status == models.StatusVerified {
			return observation, nil
		}
		attempts = append(attempts, observation)
	}

	merged := mergeObservations(attempts)
	merged.Mode = v.Name()
	return merged, nil
}

func (v *CosignAttestationVerifier) verifyType(ctx context.Context, ref, attestationType string) (models.VerificationObservation, error) {
	args := []string{
		"verify-attestation",
		"--output", "json",
		"--type", attestationType,
		"--certificate-identity-regexp", ".*",
		"--certificate-oidc-issuer-regexp", ".*",
	}
	args = append(args, v.registryCreds.CosignArgs()...)
	args = append(args, ref)

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
			Reason: fmt.Sprintf("cosign attestation verification succeeded but signer data could not be parsed: %v", err),
		}, nil
	}

	if len(signers) == 0 {
		return models.VerificationObservation{
			Mode:   v.Name(),
			Status: models.StatusVerificationError,
			Reason: "cosign attestation verification succeeded but signer identity could not be determined",
		}, nil
	}
	for _, signer := range signers {
		if v.isTrustedSigner(signer) {
			return models.VerificationObservation{
				Mode:            v.Name(),
				Status:          models.StatusVerified,
				Reason:          fmt.Sprintf("cosign attestation verification succeeded for type %s", attestationType),
				AttestationType: attestationType,
				Signer:          signer,
				Signers:         signers,
			}, nil
		}
	}
	return models.VerificationObservation{
		Mode:    v.Name(),
		Status:  models.StatusSignedUntrusted,
		Reason:  fmt.Sprintf("attestation type %s was verified but no signer matched the configured trust policy", attestationType),
		Signer:  signers[0],
		Signers: signers,
	}, nil
}

func (v *CosignAttestationVerifier) hasTrustPolicy() bool {
	return v.trustedIssuerMatcher != nil || v.trustedSubjectMatcher != nil
}

func (v *CosignAttestationVerifier) isTrustedSigner(signer models.SignerDetails) bool {
	if v.trustedIssuerMatcher != nil && !v.trustedIssuerMatcher.MatchString(signer.Issuer) {
		return false
	}
	if v.trustedSubjectMatcher != nil && !v.trustedSubjectMatcher.MatchString(signer.Subject) {
		return false
	}
	return true
}
