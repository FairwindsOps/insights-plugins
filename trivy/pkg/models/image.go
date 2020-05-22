package models

import "time"

type Image struct {
	Name    string
	ID      string
	PullRef string
	Owner   Resource
}
type Resource struct {
	Kind      string
	Namespace string
	Name      string
}

type ImageReport struct {
	Name      string
	ID        string
	PullRef   string
	OwnerKind string
	OwnerName string
	Namespace string
	Report    []VulnerabilityList
}

type VulnerabilityList struct {
	Target          string
	Vulnerabilities []Vulnerability
}

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

type MinimizedReport struct {
	Images          []ImageDetailsWithRefs
	Vulnerabilities map[string]VulnerabilityDetails
}

type ImageDetailsWithRefs struct {
	ID        string
	Name      string
	OwnerName string
	OwnerKind string
	Namespace string
	LastScan  *time.Time
	Report    []VulnerabilityRefList
}

type VulnerabilityRefList struct {
	Target          string
	Vulnerabilities []VulnerabilityInstance
}

type VulnerabilityDetails struct {
	Title           string
	Description     string
	Severity        string
	VulnerabilityID string
	References      []string
}

type VulnerabilityInstance struct {
	InstalledVersion string
	PkgName          string
	VulnerabilityID  string
}
