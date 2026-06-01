package verify

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/registry"
	"github.com/stretchr/testify/require"
)

type countingVerifier struct {
	calls int32
}

func (c *countingVerifier) Name() models.VerificationMode {
	return models.VerificationModeCosignKeyless
}

func (c *countingVerifier) Verify(_ context.Context, image models.DiscoveredImage) (models.VerificationObservation, error) {
	atomic.AddInt32(&c.calls, 1)
	return models.VerificationObservation{
		Mode:   c.Name(),
		Status: models.StatusVerified,
		Reason: image.Name,
	}, nil
}

func TestVerifyImagesPreservesOrder(t *testing.T) {
	verifier := &countingVerifier{}
	images := []models.DiscoveredImage{
		{Name: "first", ID: "first@sha256:1"},
		{Name: "second", ID: "second@sha256:2"},
		{Name: "third", ID: "third@sha256:3"},
	}

	results, err := VerifyImages(context.Background(), images, registry.Credentials{}, verifier, 2, time.Minute, time.Second, false, 1)
	require.NoError(t, err)
	require.Len(t, results, 3)
	require.Equal(t, "first", results[0].Name)
	require.Equal(t, "second", results[1].Name)
	require.Equal(t, "third", results[2].Name)
	require.Equal(t, int32(3), atomic.LoadInt32(&verifier.calls))
}

type multiSignerVerifier struct{}

func (multiSignerVerifier) Name() models.VerificationMode {
	return models.VerificationModeCosignKeyless
}

func (multiSignerVerifier) Verify(context.Context, models.DiscoveredImage) (models.VerificationObservation, error) {
	trusted := models.SignerDetails{
		Issuer:  "https://token.actions.githubusercontent.com",
		Subject: "https://github.com/example/repo/.github/workflows/build.yml@refs/heads/main",
	}
	untrusted := models.SignerDetails{
		Issuer:  "https://accounts.google.com",
		Subject: "keyless@projectsigstore.iam.gserviceaccount.com",
	}
	return models.VerificationObservation{
		Mode:   models.VerificationModeCosignKeyless,
		Status: models.StatusVerified,
		Signer: trusted,
		Signers: []models.SignerDetails{
			untrusted,
			trusted,
		},
	}, nil
}

func TestVerifyImagesMapsSignerAndCandidateSignersSeparately(t *testing.T) {
	images := []models.DiscoveredImage{
		{Name: "ghcr.io/example/api:1.0.0", ID: "ghcr.io/example/api@sha256:abc"},
	}

	results, err := VerifyImages(
		context.Background(),
		images,
		registry.Credentials{},
		multiSignerVerifier{},
		1,
		time.Minute,
		time.Second,
		false,
		1,
	)
	require.NoError(t, err)
	require.Len(t, results, 1)

	require.Equal(t, "https://github.com/example/repo/.github/workflows/build.yml@refs/heads/main", results[0].Signer.Subject)
	require.Len(t, results[0].CandidateSigners, 2)
	require.Equal(t, "keyless@projectsigstore.iam.gserviceaccount.com", results[0].CandidateSigners[0].Subject)
	require.Equal(t, results[0].Signer.Subject, results[0].CandidateSigners[1].Subject)
	require.NotEqual(t, results[0].Signer.Subject, results[0].CandidateSigners[0].Subject)
}

type failingVerifier struct {
	name models.VerificationMode
}

func (f failingVerifier) Name() models.VerificationMode {
	return f.name
}

func (f failingVerifier) Verify(context.Context, models.DiscoveredImage) (models.VerificationObservation, error) {
	return models.VerificationObservation{}, fmt.Errorf("boom")
}

func TestVerifyImagesRecordsPerImageVerificationError(t *testing.T) {
	images := []models.DiscoveredImage{
		{Name: "first", ID: "first@sha256:1"},
		{Name: "second", ID: "second@sha256:2"},
	}

	results, err := VerifyImages(
		context.Background(),
		images,
		registry.Credentials{},
		failingVerifier{name: models.VerificationModeCosignKeyless},
		2,
		time.Minute,
		time.Second,
		false,
		1,
	)
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Equal(t, models.StatusVerificationError, results[0].Status)
	require.Equal(t, models.StatusVerificationError, results[1].Status)
	require.Contains(t, results[0].Reason, "boom")
}

func TestVerificationModeFromObservationPrefersVerifiedBy(t *testing.T) {
	composite, err := NewCompositeVerifier("any",
		stubVerifier{
			name: models.VerificationModeCosignKeyless,
			result: models.VerificationObservation{
				Mode:   models.VerificationModeCosignKeyless,
				Status: models.StatusUnsigned,
			},
		},
		stubVerifier{
			name: models.VerificationModeCosignKey,
			result: models.VerificationObservation{
				Mode:       models.VerificationModeCosignKey,
				Status:     models.StatusVerified,
				VerifiedBy: models.VerificationModeCosignKey,
			},
		},
	)
	require.NoError(t, err)

	results, err := VerifyImages(
		context.Background(),
		[]models.DiscoveredImage{{Name: "ghcr.io/example/api:1.0.0", ID: "ghcr.io/example/api@sha256:abc"}},
		registry.Credentials{},
		composite,
		1,
		time.Minute,
		time.Second,
		false,
		1,
	)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "cosign-key", results[0].VerificationMode)
	require.Equal(t, "cosign-key", results[0].VerifiedBy)
}
