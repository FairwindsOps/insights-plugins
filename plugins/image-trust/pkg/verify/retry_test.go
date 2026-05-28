package verify

import (
	"context"
	"testing"
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/registry"
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
	observation, err := VerifyWithRetries(context.Background(), verifier, models.DiscoveredImage{Name: "img:1"}, 3, 10*time.Millisecond, false)
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
	observation, err := VerifyWithRetries(context.Background(), verifier, models.DiscoveredImage{}, 3, time.Second, false)
	require.NoError(t, err)
	require.Equal(t, models.StatusUnsigned, observation.Status)
}

func TestVerifyWithRetriesRespectsContext(t *testing.T) {
	verifier := &flakyVerifier{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := VerifyWithRetries(ctx, verifier, models.DiscoveredImage{}, 3, time.Second, false)
	require.Error(t, err)
}

func TestPreflightDigestResolveError(t *testing.T) {
	image := models.DiscoveredImage{
		Name:               "private.registry/app:1.0",
		DigestResolveError: "registry digest lookup failed: unauthorized",
	}
	observation, stop := Preflight(image, registry.Credentials{})
	require.True(t, stop)
	require.Equal(t, models.StatusVerificationError, observation.Status)
	require.Contains(t, observation.Reason, "digest lookup failed")
}
