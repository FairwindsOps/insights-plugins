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

func TestCompositeVerifierAllRequiresEveryMode(t *testing.T) {
	composite, err := NewCompositeVerifier("all",
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
				Status: models.StatusUnsigned,
				Reason: "no matching signatures",
			},
		},
	)
	require.NoError(t, err)

	observation, err := composite.Verify(context.Background(), models.DiscoveredImage{})
	require.NoError(t, err)
	require.Equal(t, models.StatusUnsigned, observation.Status)
}

func TestCompositeVerifierAllSucceedsWhenAllVerify(t *testing.T) {
	composite, err := NewCompositeVerifier("all",
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
	require.Equal(t, models.StatusVerified, observation.Status)
	require.Contains(t, observation.Reason, "all configured verification modes succeeded")
}

func TestCompositeVerifierAllMergesAttestationTypeWhenAttestationRunsFirst(t *testing.T) {
	composite, err := NewCompositeVerifier("all",
		stubVerifier{
			name: models.VerificationModeCosignAttestationKeyless,
			result: models.VerificationObservation{
				Mode:            models.VerificationModeCosignAttestationKeyless,
				Status:          models.StatusVerified,
				AttestationType: "slsaprovenance1",
				Signer: models.SignerDetails{
					Subject: "https://github.com/example/workflow",
				},
			},
		},
		stubVerifier{
			name: models.VerificationModeCosignKeyless,
			result: models.VerificationObservation{
				Mode:   models.VerificationModeCosignKeyless,
				Status: models.StatusVerified,
				Signer: models.SignerDetails{
					Issuer:  "https://token.actions.githubusercontent.com",
					Subject: "https://github.com/example/repo/.github/workflows/release.yml@refs/heads/main",
				},
			},
		},
	)
	require.NoError(t, err)

	observation, err := composite.Verify(context.Background(), models.DiscoveredImage{})
	require.NoError(t, err)
	require.Equal(t, models.StatusVerified, observation.Status)
	require.Equal(t, "slsaprovenance1", observation.AttestationType)
	require.Equal(t, "https://token.actions.githubusercontent.com", observation.Signer.Issuer)
}

func TestCompositeVerifierAllMergesAttestationTypeWhenSignatureRunsFirst(t *testing.T) {
	composite, err := NewCompositeVerifier("all",
		stubVerifier{
			name: models.VerificationModeCosignKeyless,
			result: models.VerificationObservation{
				Mode:   models.VerificationModeCosignKeyless,
				Status: models.StatusVerified,
				Signer: models.SignerDetails{
					Issuer:  "https://token.actions.githubusercontent.com",
					Subject: "https://github.com/example/repo/.github/workflows/release.yml@refs/heads/main",
				},
			},
		},
		stubVerifier{
			name: models.VerificationModeCosignAttestationKeyless,
			result: models.VerificationObservation{
				Mode:            models.VerificationModeCosignAttestationKeyless,
				Status:          models.StatusVerified,
				AttestationType: "spdxjson",
				Signer: models.SignerDetails{
					Subject: "https://github.com/example/sbom",
				},
			},
		},
	)
	require.NoError(t, err)

	observation, err := composite.Verify(context.Background(), models.DiscoveredImage{})
	require.NoError(t, err)
	require.Equal(t, models.StatusVerified, observation.Status)
	require.Equal(t, "spdxjson", observation.AttestationType)
	require.Equal(t, "https://token.actions.githubusercontent.com", observation.Signer.Issuer)
}
