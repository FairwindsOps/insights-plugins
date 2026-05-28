package verify

import (
	"context"
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/sirupsen/logrus"
)

const retryBackoff = 2 * time.Second

// VerifyWithRetries runs verification and retries transient operational failures.
func VerifyWithRetries(
	ctx context.Context,
	verifier Verifier,
	image models.DiscoveredImage,
	retries int,
) (models.VerificationObservation, error) {
	if retries < 1 {
		retries = 1
	}

	var last models.VerificationObservation
	for attempt := 1; attempt <= retries; attempt++ {
		observation, err := verifier.Verify(ctx, image)
		if err != nil {
			return models.VerificationObservation{}, err
		}
		last = observation
		if observation.Status != models.StatusVerificationError || !IsTransientFailure(observation.Reason) {
			return observation, nil
		}
		if attempt == retries {
			break
		}
		logrus.Warnf("retrying transient verification failure for %s (attempt %d/%d): %s",
			image.Name, attempt, retries, observation.Reason)
		select {
		case <-ctx.Done():
			return models.VerificationObservation{}, ctx.Err()
		case <-time.After(retryBackoff):
		}
	}
	return last, nil
}
