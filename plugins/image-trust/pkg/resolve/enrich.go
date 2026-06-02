package resolve

import (
	"context"
	"sync"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/registry"
)

// digestEnricher resolves tag-only images to digest-backed references.
var digestEnricher = registry.EnrichDigest

// Images resolves tag-only discovered images to digest references when enabled.
func Images(ctx context.Context, creds registry.Credentials, images []models.DiscoveredImage, enabled bool, maxConcurrent int) []models.DiscoveredImage {
	if !enabled || len(images) == 0 {
		return images
	}
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}

	enriched := make([]models.DiscoveredImage, len(images))
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrent)

	for i, image := range images {
		wg.Add(1)
		sem <- struct{}{}
		go func(index int, img models.DiscoveredImage) {
			defer wg.Done()
			defer func() { <-sem }()
			enriched[index] = digestEnricher(ctx, creds, img)
		}(i, image)
	}

	wg.Wait()
	return enriched
}
