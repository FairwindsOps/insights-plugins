package verify

import (
	"context"
	"math/rand"
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/sirupsen/logrus"
)

// VerifyWithRetries runs verification and retries transient operational failures.
func VerifyWithRetries(
	ctx context.Context,
	verifier Verifier,
	image models.DiscoveredImage,
	retries int,
	backoff time.Duration,
	jitter bool,
) (models.VerificationObservation, error) {
	if retries < 1 {
		retries = 1
	}
	if backoff <= 0 {
		backoff = 2 * time.Second
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
		delay := backoff
		if jitter {
			delay = backoff + time.Duration(rand.Int63n(int64(backoff)))
		}
		logrus.Warnf("retrying transient verification failure for %s (attempt %d/%d, sleep %s): %s",
			image.Name, attempt, retries, delay, observation.Reason)
		select {
		case <-ctx.Done():
			return models.VerificationObservation{}, ctx.Err()
		case <-time.After(delay):
		}
	}
	return last, nil
}
