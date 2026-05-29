package resolve

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/registry"
	"github.com/stretchr/testify/require"
)

func TestImagesDisabledReturnsInput(t *testing.T) {
	images := []models.DiscoveredImage{{Name: "example:latest"}}
	got := Images(context.Background(), registry.Credentials{}, images, false, 4)
	require.Equal(t, images, got)
}

func TestImagesRunsConcurrently(t *testing.T) {
	original := digestEnricher
	t.Cleanup(func() {
		digestEnricher = original
	})

	var active int32
	var maxActive int32
	digestEnricher = func(_ context.Context, _ registry.Credentials, image models.DiscoveredImage) models.DiscoveredImage {
		current := atomic.AddInt32(&active, 1)
		for {
			prev := atomic.LoadInt32(&maxActive)
			if current <= prev || atomic.CompareAndSwapInt32(&maxActive, prev, current) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt32(&active, -1)
		return image
	}

	images := make([]models.DiscoveredImage, 4)
	for i := range images {
		images[i] = models.DiscoveredImage{Name: "example:latest"}
	}

	got := Images(context.Background(), registry.Credentials{}, images, true, 4)
	require.Len(t, got, 4)
	require.GreaterOrEqual(t, atomic.LoadInt32(&maxActive), int32(2))
}
