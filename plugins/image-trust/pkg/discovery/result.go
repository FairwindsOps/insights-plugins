package discovery

import (
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/registry"
)

// Result contains images discovered in scope and pull secrets referenced by those workloads.
type Result struct {
	Images         []models.DiscoveredImage
	PullSecretRefs []registry.PullSecretRef
}
