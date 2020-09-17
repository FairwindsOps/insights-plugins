package models

import "time"

// Image represents a single container image to scan.
type Image struct {
	Name    string
	ID      string
	PullRef string
	Owner   Resource
}

// Resource represents a Kubernetes resource
type Resource struct {
	Kind      string
	Namespace string
	Name      string
	Container string
}

// ImageReport represents the results for a single resource.
type ImageReport struct {
	Name           string
	ID             string
	PullRef        string
	OwnerKind      string
	OwnerName      string
	OwnerContainer *string
	Namespace      string
	Report         []VulnerabilityList
}

// VulnerabilityList is the results from Trivy
type VulnerabilityList struct {
	Target          string
	Vulnerabilities []Vulnerability
}

// Vulnerability is a single CVE or vulnerability.
type Vulnerability struct {
	Title            string
	Description      string
	InstalledVersion string
	FixedVersion     string
	PkgName          string
	Severity         string
	VulnerabilityID  string
	References       []string
}

// MinimizedReport is the results in a compressed format.
type MinimizedReport struct {
	Images          []ImageDetailsWithRefs
	Vulnerabilities map[string]VulnerabilityDetails
}

// ImageDetailsWithRefs is the results of a scan for a resource with the vulnerabilities replaced with references.
type ImageDetailsWithRefs struct {
	ID             string
	Name           string
	OwnerName      string
	OwnerKind      string
	OwnerContainer *string
	Namespace      string
	LastScan       *time.Time
	Report         []VulnerabilityRefList
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
}
