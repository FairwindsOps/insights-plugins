package discovery

import (
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
)

// Result contains images discovered in scope.
type Result struct {
	Images []models.DiscoveredImage
}
