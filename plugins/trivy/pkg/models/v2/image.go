package v2

import (
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/models"
)

// MinimizedReport is the results in a compressed format.
type MinimizedReport struct {
	Images          []ImageDetailsWithRefs
	Vulnerabilities map[string]VulnerabilityDetails
}

// Resource represents a Kubernetes resource
type Resource struct {
	Kind      string
	Namespace string
	Name      string
	Container string
}

// ImageDetailsWithRefs is the results of a scan for a resource with the vulnerabilities replaced with references.
type ImageDetailsWithRefs struct {
	ID                 string
	Name               string
	OSArch             string
	Owners             []Resource
	LastScan           *time.Time
	Report             []VulnerabilityRefList
	RecommendationOnly bool
}

// VulnerabilityRefList is a list of vulnerability references.
type VulnerabilityRefList struct {
	Target          string
	Vulnerabilities []VulnerabilityInstance
}

// VulnerabilityDetails are the details of a vulnerability itself.
type VulnerabilityDetails struct {
	Title           string
	Description     string
	Severity        string
	VulnerabilityID string
	References      []string
}

// VulnerabilityInstance is a single instance of a given vulnerability
type VulnerabilityInstance struct {
	InstalledVersion string
	PkgName          string
	VulnerabilityID  string
	FixedVersion     string
}

func (i ImageDetailsWithRefs) GetSha() string {
	return models.GetShaFromID(i.ID)
}

// GetUniqueID returns a unique ID for the image
func (i ImageDetailsWithRefs) GetUniqueID() string {
	return models.GetUniqueID(i.Name, i.ID)
}
