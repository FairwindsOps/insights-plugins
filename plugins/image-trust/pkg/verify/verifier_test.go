package verify

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
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

	results, err := VerifyImages(context.Background(), images, verifier, 2, time.Minute)
	require.NoError(t, err)
	require.Len(t, results, 3)
	require.Equal(t, "first", results[0].Name)
	require.Equal(t, "second", results[1].Name)
	require.Equal(t, "third", results[2].Name)
	require.Equal(t, int32(3), atomic.LoadInt32(&verifier.calls))
}
