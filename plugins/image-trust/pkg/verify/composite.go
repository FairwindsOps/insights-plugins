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
	observation := attempts[len(attempts)-1]
	if observation.VerifiedBy == "" {
		observation.VerifiedBy = observation.Mode
	}
	observation.Reason = "all configured verification modes succeeded"
	return observation, nil
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
