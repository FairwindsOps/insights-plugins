package verify

import (
	"context"
	"testing"
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/stretchr/testify/require"
)

type flakyVerifier struct {
	attempts int
}

func (f *flakyVerifier) Name() models.VerificationMode {
	return models.VerificationModeCosignKeyless
}

func (f *flakyVerifier) Verify(context.Context, models.DiscoveredImage) (models.VerificationObservation, error) {
	f.attempts++
	if f.attempts < 2 {
		return models.VerificationObservation{
			Mode:   f.Name(),
			Status: models.StatusVerificationError,
			Reason: "Get \"https://rekor.sigstore.dev\": dial tcp i/o timeout",
		}, nil
	}
	return models.VerificationObservation{
		Mode:   f.Name(),
		Status: models.StatusVerified,
	}, nil
}

func TestVerifyWithRetriesRecoversFromTransientFailure(t *testing.T) {
	verifier := &flakyVerifier{}
	observation, err := VerifyWithRetries(context.Background(), verifier, models.DiscoveredImage{Name: "img:1"}, 3)
	require.NoError(t, err)
	require.Equal(t, models.StatusVerified, observation.Status)
	require.Equal(t, 2, verifier.attempts)
}

func TestVerifyWithRetriesDoesNotRetryUnsigned(t *testing.T) {
	verifier := &stubVerifier{
		name: models.VerificationModeCosignKeyless,
		result: models.VerificationObservation{
			Mode:   models.VerificationModeCosignKeyless,
			Status: models.StatusUnsigned,
		},
	}
	observation, err := VerifyWithRetries(context.Background(), verifier, models.DiscoveredImage{}, 3)
	require.NoError(t, err)
	require.Equal(t, models.StatusUnsigned, observation.Status)
}

func TestVerifyWithRetriesRespectsContext(t *testing.T) {
	verifier := &flakyVerifier{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	_, err := VerifyWithRetries(ctx, verifier, models.DiscoveredImage{}, 3)
	require.Error(t, err)
}
