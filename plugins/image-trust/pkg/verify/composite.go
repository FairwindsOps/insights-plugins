package verify

import (
	"context"
	"fmt"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/config"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
)

// CompositeVerifier runs multiple verification strategies and merges results.
type CompositeVerifier struct {
	verifiers []Verifier
	policy    string
}

// NewCompositeVerifier creates a verifier that applies the configured mode policy.
func NewCompositeVerifier(policy string, verifiers ...Verifier) (*CompositeVerifier, error) {
	if len(verifiers) == 0 {
		return nil, fmt.Errorf("at least one verifier is required")
	}
	if policy == "" {
		policy = config.ModePolicyAny
	}
	if policy != config.ModePolicyAny && policy != config.ModePolicyAll {
		return nil, fmt.Errorf("unsupported mode policy %q", policy)
	}
	return &CompositeVerifier{
		verifiers: append([]Verifier(nil), verifiers...),
		policy:    policy,
	}, nil
}

func (c *CompositeVerifier) Name() models.VerificationMode {
	return c.verifiers[0].Name()
}

func (c *CompositeVerifier) Verify(ctx context.Context, image models.DiscoveredImage) (models.VerificationObservation, error) {
	switch c.policy {
	case config.ModePolicyAny:
		return c.verifyAny(ctx, image)
	case config.ModePolicyAll:
		return c.verifyAll(ctx, image)
	default:
		return models.VerificationObservation{}, fmt.Errorf("unsupported mode policy %q", c.policy)
	}
}

func (c *CompositeVerifier) verifyAll(ctx context.Context, image models.DiscoveredImage) (models.VerificationObservation, error) {
	attempts := make([]models.VerificationObservation, 0, len(c.verifiers))
	for _, verifier := range c.verifiers {
		observation, err := verifier.Verify(ctx, image)
		if err != nil {
			return models.VerificationObservation{}, fmt.Errorf("%s: %w", verifier.Name(), err)
		}
		if observation.Status != models.StatusVerified {
			if observation.VerifiedBy == "" {
				observation.VerifiedBy = observation.Mode
			}
			return observation, nil
		}
		attempts = append(attempts, observation)
	}
	if len(attempts) == 0 {
		return models.VerificationObservation{
			Status: models.StatusVerificationError,
			Reason: "no verification modes produced a result",
		}, nil
	}
	return mergeVerifiedObservations(attempts), nil
}

func mergeVerifiedObservations(attempts []models.VerificationObservation) models.VerificationObservation {
	merged := attempts[len(attempts)-1]
	if merged.VerifiedBy == "" {
		merged.VerifiedBy = merged.Mode
	}

	for _, attempt := range attempts {
		if attempt.AttestationType != "" {
			merged.AttestationType = attempt.AttestationType
		}
	}

	for _, attempt := range attempts {
		if isSignatureMode(attempt.Mode) && !signerIsEmpty(attempt.Signer) {
			merged.Signer = attempt.Signer
			break
		}
	}
	if signerIsEmpty(merged.Signer) {
		for _, attempt := range attempts {
			if isAttestationMode(attempt.Mode) && !signerIsEmpty(attempt.Signer) {
				merged.Signer = attempt.Signer
				break
			}
		}
	}

	allSigners := make([]models.SignerDetails, 0)
	for _, attempt := range attempts {
		allSigners = append(allSigners, attempt.Signers...)
	}
	merged.Signers = dedupeSigners(allSigners)
	merged.Reason = "all configured verification modes succeeded"
	return merged
}

func isSignatureMode(mode models.VerificationMode) bool {
	switch mode {
	case models.VerificationModeCosignKeyless, models.VerificationModeCosignKey:
		return true
	default:
		return false
	}
}

func isAttestationMode(mode models.VerificationMode) bool {
	switch mode {
	case models.VerificationModeCosignAttestationKeyless, models.VerificationModeCosignAttestationKey:
		return true
	default:
		return false
	}
}

func signerIsEmpty(signer models.SignerDetails) bool {
	return signer.Issuer == "" && signer.Subject == "" && signer.KeyRef == ""
}

func dedupeSigners(signers []models.SignerDetails) []models.SignerDetails {
	if len(signers) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(signers))
	deduped := make([]models.SignerDetails, 0, len(signers))
	for _, signer := range signers {
		if signerIsEmpty(signer) {
			continue
		}
		key := signer.Issuer + "\x00" + signer.Subject + "\x00" + signer.KeyRef
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, signer)
	}
	return deduped
}

func (c *CompositeVerifier) verifyAny(ctx context.Context, image models.DiscoveredImage) (models.VerificationObservation, error) {
	attempts := make([]models.VerificationObservation, 0, len(c.verifiers))
	for _, verifier := range c.verifiers {
		observation, err := verifier.Verify(ctx, image)
		if err != nil {
			return models.VerificationObservation{}, fmt.Errorf("%s: %w", verifier.Name(), err)
		}
		if observation.Status == models.StatusVerified {
			if observation.VerifiedBy == "" {
				observation.VerifiedBy = observation.Mode
			}
			return observation, nil
		}
		attempts = append(attempts, observation)
	}
	merged := mergeObservations(attempts)
	if merged.VerifiedBy == "" && merged.Mode == "" && len(attempts) > 0 {
		merged.Mode = attempts[0].Mode
	}
	return merged, nil
}

func mergeObservations(attempts []models.VerificationObservation) models.VerificationObservation {
	if len(attempts) == 0 {
		return models.VerificationObservation{
			Status: models.StatusVerificationError,
			Reason: "no verification modes produced a result",
		}
	}

	priority := []models.Status{
		models.StatusSignedUntrusted,
		models.StatusUnsigned,
		models.StatusVerificationError,
		models.StatusUnknown,
	}
	for _, status := range priority {
		for _, attempt := range attempts {
			if attempt.Status == status {
				observation := attempt
				if observation.VerifiedBy == "" {
					observation.VerifiedBy = observation.Mode
				}
				return observation
			}
		}
	}

	observation := attempts[0]
	if observation.VerifiedBy == "" {
		observation.VerifiedBy = observation.Mode
	}
	return observation
}
