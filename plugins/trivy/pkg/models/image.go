package models

import (
	"strings"
	"time"
)

type DockerImage struct {
	Name    string // paulbouwer/hello-kubernetes:1.7
	PullRef string // paulbouwerhellokubernetes17
}

// Image represents a single container image to scan.
type Image struct {
	Name               string // paulbouwer/hello-kubernetes:1.7
	ID                 string // paulbouwer/hello-kubernetes@sha256:93b15e948cae979539e152659edfd16549e3009140cc8a9ea2b91ffbd80a07f6
	PullRef            string // paulbouwerhellokubernetes17
	Owner              Resource
	RecommendationOnly bool
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
	ID                 string
	Name               string
	OSArch             string
	PullRef            string
	OwnerKind          string
	OwnerName          string
	OwnerContainer     *string
	Namespace          string
	Reports            []VulnerabilityList `json:"Report"`
	RecommendationOnly bool
}

type TrivyResults struct {
	Metadata TrivyMetadata
	Results  []VulnerabilityList
}

type TrivyMetadata struct {
	ImageID     string
	RepoDigests []string
	ImageConfig TrivyImageConfig
}

type TrivyImageConfig struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
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
	ID                 string
	Name               string
	OSArch             string
	OwnerName          string
	OwnerKind          string
	OwnerContainer     *string
	Namespace          string
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

func getShaFromID(id string) string {
	if len(strings.Split(id, "@")) > 1 {
		return strings.Split(id, "@")[1]
	}
	return id
}

func (i Image) GetSha() string {
	return getShaFromID(i.ID)
}

func (i ImageDetailsWithRefs) GetSha() string {
	return getShaFromID(i.ID)
}

func (i ImageReport) GetSha() string {
	return getShaFromID(i.ID)
}

func getUniqueID(name string, id string) string {
	if id != "" {
		return getShaFromID(id)
	} else {
		return name + "@" // FIXME: this is kind of a hack. This is what image IDs end up looking like in the report if there's no ID reported by k8s (e.g. image couldn't pull)
	}
}

// GetUniqueID returns a unique ID for the image
func (i Image) GetUniqueID() string {
	return getUniqueID(i.Name, i.ID)
}

// GetUniqueID returns a unique ID for the image
func (i ImageDetailsWithRefs) GetUniqueID() string {
	return getUniqueID(i.Name, i.ID)
}
