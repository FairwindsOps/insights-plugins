package verify

import (
	"context"
	"testing"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/stretchr/testify/require"
)

type stubVerifier struct {
	name   models.VerificationMode
	result models.VerificationObservation
	err    error
}

func (s stubVerifier) Name() models.VerificationMode {
	return s.name
}

func (s stubVerifier) Verify(context.Context, models.DiscoveredImage) (models.VerificationObservation, error) {
	return s.result, s.err
}

func TestCompositeVerifierAnyReturnsFirstVerified(t *testing.T) {
	composite, err := NewCompositeVerifier("any",
		stubVerifier{
			name: models.VerificationModeCosignKeyless,
			result: models.VerificationObservation{
				Mode:   models.VerificationModeCosignKeyless,
				Status: models.StatusUnsigned,
				Reason: "no signatures",
			},
		},
		stubVerifier{
			name: models.VerificationModeCosignKey,
			result: models.VerificationObservation{
				Mode:   models.VerificationModeCosignKey,
				Status: models.StatusVerified,
				Reason: "matched release.pub",
			},
		},
	)
	require.NoError(t, err)

	observation, err := composite.Verify(context.Background(), models.DiscoveredImage{
		Name: "ghcr.io/example/api:1.0.0",
		ID:   "ghcr.io/example/api@sha256:abc",
	})
	require.NoError(t, err)
	require.Equal(t, models.StatusVerified, observation.Status)
	require.Equal(t, models.VerificationModeCosignKey, observation.VerifiedBy)
}

func TestCompositeVerifierAnyPrefersKeylessWhenBothVerify(t *testing.T) {
	composite, err := NewCompositeVerifier("any",
		stubVerifier{
			name: models.VerificationModeCosignKeyless,
			result: models.VerificationObservation{
				Mode:   models.VerificationModeCosignKeyless,
				Status: models.StatusVerified,
			},
		},
		stubVerifier{
			name: models.VerificationModeCosignKey,
			result: models.VerificationObservation{
				Mode:   models.VerificationModeCosignKey,
				Status: models.StatusVerified,
			},
		},
	)
	require.NoError(t, err)

	observation, err := composite.Verify(context.Background(), models.DiscoveredImage{})
	require.NoError(t, err)
	require.Equal(t, models.VerificationModeCosignKeyless, observation.VerifiedBy)
}

func TestCompositeVerifierAnyMergesFailures(t *testing.T) {
	composite, err := NewCompositeVerifier("any",
		stubVerifier{
			name: models.VerificationModeCosignKeyless,
			result: models.VerificationObservation{
				Mode:   models.VerificationModeCosignKeyless,
				Status: models.StatusVerificationError,
				Reason: "registry denied",
			},
		},
		stubVerifier{
			name: models.VerificationModeCosignKey,
			result: models.VerificationObservation{
				Mode:   models.VerificationModeCosignKey,
				Status: models.StatusUnsigned,
				Reason: "no matching signatures",
			},
		},
	)
	require.NoError(t, err)

	observation, err := composite.Verify(context.Background(), models.DiscoveredImage{})
	require.NoError(t, err)
	require.Equal(t, models.StatusUnsigned, observation.Status)
	require.Equal(t, "no matching signatures", observation.Reason)
}
