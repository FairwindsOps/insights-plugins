package resolve

import (
	"context"

	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/registry"
)

// Images resolves tag-only discovered images to digest references when enabled.
func Images(ctx context.Context, creds registry.Credentials, images []models.DiscoveredImage, enabled bool) []models.DiscoveredImage {
	if !enabled || len(images) == 0 {
		return images
	}

	enriched := make([]models.DiscoveredImage, len(images))
	for i, image := range images {
		enriched[i] = registry.EnrichDigest(ctx, creds, image)
	}
	return enriched
}
