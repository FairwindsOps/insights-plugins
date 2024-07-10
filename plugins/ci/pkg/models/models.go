package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

// Resource represents a Kubernetes resource with information about what file it came from.
type Resource struct {
	Kind       string
	Name       string
	Filename   string
	Namespace  string
	HelmName   string
	Labels     map[string]string
	Containers []string
}

// ReportInfo is the information about a run of one of the reports.
type ReportInfo struct {
	Report   string
	Version  string
	Filename string
}

// Configuration is a struct representing the config options for Insights CI/CD
type Configuration struct {
	Images    imageConfig     `yaml:"images"`
	Manifests ManifestConfig  `yaml:"manifests"`
	Terraform TerraformConfig `yaml:"terraform"`
	Options   optionConfig    `yaml:"options"`
	Reports   reportsConfig   `yaml:"reports"`
}

func (c *Configuration) String() string {
	if c == nil {
		return "nil"
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Sprintf("error marshalling config: %v", err)
	}
	return string(b)
}

// ManifestConfig is a struct representing the config options for Manifests
type ManifestConfig struct {
	YamlPaths []string     `yaml:"yaml"`
	Helm      []HelmConfig `yaml:"helm"`
}

// TerraformConfig is a struct representing the config options for Terraform
type TerraformConfig struct {
	Paths []string `yaml:"paths"`
}

// HelmConfig is the configuration for helm.
type HelmConfig struct {
	Name        string                 `yaml:"name"`
	Path        string                 `yaml:"path"`
	Repo        string                 `yaml:"repo"`
	Chart       string                 `yaml:"chart"`
	FluxFile    string                 `yaml:"fluxFile"`
	Version     string                 `yaml:"version"`
	ValuesFile  string                 `yaml:"valuesFile"` // Deprecated
	ValuesFiles []string               `yaml:"valuesFiles"`
	Values      map[string]interface{} `yaml:"values"`
}

func (hc *HelmConfig) IsRemote() bool {
	return hc.Repo != ""
}

func (hc *HelmConfig) IsLocal() bool {
	return hc.Path != ""
}

func (hc *HelmConfig) IsFluxFile() bool {
	return hc.FluxFile != ""
}

type reportsConfig struct {
	Polaris           reportConfig `yaml:"polaris"`
	Pluto             reportConfig `yaml:"pluto"`
	TFSec             tfSecConfig  `yaml:"tfsec"`
	Trivy             trivyConfig  `yaml:"trivy"`
	OPA               reportConfig `yaml:"opa"`
	PrometheusMetrics reportConfig `yaml:"prometheus-metrics"`
	Goldilocks        reportConfig `yaml:"goldilocks"`
}

type reportConfig struct {
	Enabled *bool `yaml:"enabled"`
}

type tfSecConfig struct {
	Enabled               *bool   `yaml:"enabled"`
	CustomChecksDirectory *string `yaml:"customChecksDirectory"`
}

type trivyConfig struct {
	Enabled       *bool `yaml:"enabled"`
	SkipManifests *bool `yaml:"skipManifests"`
}

type CIRunnerVal string

const (
	GithubActions CIRunnerVal = "github-actions"
	CircleCI      CIRunnerVal = "circle-ci"
	Gitlab        CIRunnerVal = "gitlab"
	Travis        CIRunnerVal = "travis"
	AzureDevops   CIRunnerVal = "azure-devops"
)

type RegistryCredential struct {
	Domain   string `yaml:"domain"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

func (rc RegistryCredential) String() string {
	return fmt.Sprintf("domain: %s, username: %s, password: %s", rc.Domain, rc.Username, strings.Map(func(r rune) rune { return '*' }, rc.Password))
}

type RegistryCredentials []RegistryCredential

func (rc RegistryCredentials) Validate() error {
	// make sure there is no duplicated domain
	domains := map[string]struct{}{}
	for _, v := range rc {
		domains[v.Domain] = struct{}{}
	}
	if len(rc) > len(domains) {
		return errors.New("duplicated domains found in registry credentials list")
	}
	return nil
}

func (rc RegistryCredentials) FindCredentialForImage(imageName string) *RegistryCredential {
	parts := strings.Split(imageName, "/")
	domain := "docker.io"
	if len(parts) == 3 {
		// quay.io/fedora/httpd:version1.0
		domain = parts[0]
	}

	for _, v := range rc {
		if v.Domain == domain {
			return &v
		}
	}

	return nil
}

type optionConfig struct {
	SetExitCode            bool                `yaml:"setExitCode"`
	BaseBranch             string              `yaml:"baseBranch"`
	NewActionItemThreshold int                 `yaml:"newActionItemThreshold"`
	SeverityThreshold      string              `yaml:"severityThreshold"`
	TempFolder             string              `yaml:"tempFolder"`
	Hostname               string              `yaml:"hostname"`
	Organization           string              `yaml:"organization"`
	JUnitOutput            string              `yaml:"junitOutput"`
	RepositoryName         string              `yaml:"repositoryName"`
	RegistryCredentials    RegistryCredentials `yaml:"-"`
	CIRunner               CIRunnerVal         `yaml:"-"`
}

type imageConfig struct {
	FolderName string   `yaml:"folder"`
	Docker     []string `yaml:"docker"`
}

// ScanResults is the value returned by the Insights API upon submitting a scan.
type ScanResults struct {
	NewActionItems   []ActionItem
	FixedActionItems []ActionItem
	Pass             bool
}

// Container is an individual container within a pod.
type Container struct {
	Image string
	Name  string
}

// ActionItem represents an ActionItem from Insights
type ActionItem struct {
	Remediation string
	Severity    float64
	Title       string
	Description string
	Notes       string
	Resource    K8sResource
}

// K8sResource represents a resource in the cluster
type K8sResource struct {
	Namespace string
	Name      string
	Kind      string
	Filename  string
}

// GetReadableTitle returns a human-readable title for the action item
func (ai ActionItem) GetReadableTitle() string {
	t := ""
	if ai.Resource.Filename != "" {
		t += ai.Resource.Filename + ": "
	}
	if ai.Resource.Namespace == "" {
		t += fmt.Sprintf("%s/%s", ai.Resource.Kind, ai.Resource.Name)
	} else {
		t += fmt.Sprintf("%s/%s/%s", ai.Resource.Namespace, ai.Resource.Kind, ai.Resource.Name)
	}
	return t + " - " + ai.Title
}

func maybeAddSlash(input string) string {
	if strings.HasSuffix(input, "/") {
		return input
	}
	return input + "/"
}

// SetDefaults sets configuration defaults
func (c *Configuration) SetMountedPathDefaults(basePath, repoPath string) error {
	c.Options.TempFolder = filepath.Join(basePath, "tmp/_insightsTemp")
	err := os.MkdirAll(c.Options.TempFolder, os.ModePerm)
	if err != nil {
		return fmt.Errorf("SetMountedPathDefaults: %v", err)
	}
	c.Options.TempFolder = maybeAddSlash(c.Options.TempFolder)

	c.Images.FolderName = filepath.Join(basePath, "tmp/_insightsTempImages")
	err = os.MkdirAll(c.Images.FolderName, os.ModePerm)
	if err != nil {
		return fmt.Errorf("SetMountedPathDefaults: %v", err)
	}
	c.Images.FolderName = maybeAddSlash(c.Images.FolderName)
	return nil
}

// SetDefaults sets configuration defaults
func (c *Configuration) SetPathDefaults() {
	if c.Options.TempFolder == "" {
		c.Options.TempFolder = "/tmp/_insightsTemp/"
	}
	c.Options.TempFolder = maybeAddSlash(c.Options.TempFolder)
	if c.Images.FolderName == "" {
		c.Images.FolderName = "./_insightsTempImages/"
	}
	c.Images.FolderName = maybeAddSlash(c.Images.FolderName)
}

// SetDefaults sets configuration defaults
//
// it should respect the order:
// - config. file content > env. variables > default
func (c *Configuration) SetDefaults() error {
	c.Options.CIRunner = CIRunnerVal(strings.TrimSpace(os.Getenv("CI_RUNNER"))) // only set via env. variable

	if c.Options.BaseBranch == "" {
		baseBranch := strings.TrimSpace(os.Getenv("BASE_BRANCH"))
		if baseBranch != "" {
			c.Options.BaseBranch = baseBranch
		} else {
			c.Options.BaseBranch = "master"
		}
	}
	if c.Options.Organization == "" {
		c.Options.Organization = strings.TrimSpace(os.Getenv("ORG_NAME"))
	}
	if c.Options.RepositoryName == "" {
		c.Options.RepositoryName = strings.TrimSpace(os.Getenv("REPOSITORY_NAME"))
	}
	if c.Options.Hostname == "" {
		hostname := strings.TrimSpace(os.Getenv("HOSTNAME"))
		if hostname != "" {
			c.Options.Hostname = hostname
		} else {
			c.Options.Hostname = "https://insights.fairwinds.com"
		}
	}
	if c.Options.SeverityThreshold == "" {
		c.Options.SeverityThreshold = "danger"
	}
	if c.Options.NewActionItemThreshold == 0 {
		c.Options.NewActionItemThreshold = -1
	}
	truth := true
	falsehood := false
	if c.Reports.Pluto.Enabled == nil {
		c.Reports.Pluto.Enabled = &truth
	}
	if c.Reports.Polaris.Enabled == nil {
		c.Reports.Polaris.Enabled = &truth
	}
	if c.Reports.OPA.Enabled == nil {
		c.Reports.OPA.Enabled = &truth
	}
	if c.Reports.Trivy.Enabled == nil {
		c.Reports.Trivy.Enabled = &truth
	}
	if c.Reports.Trivy.SkipManifests == nil {
		c.Reports.Trivy.SkipManifests = &falsehood
	}
	if c.Reports.TFSec.Enabled == nil {
		c.Reports.TFSec.Enabled = &truth
	}
	if c.Reports.Goldilocks.Enabled == nil {
		c.Reports.Goldilocks.Enabled = &truth
	}
	if c.Reports.PrometheusMetrics.Enabled == nil {
		c.Reports.PrometheusMetrics.Enabled = &truth
	}

	registryCredentialsJSON := strings.TrimSpace(os.Getenv("REGISTRY_CREDENTIALS")) // only set via env. variable
	if registryCredentialsJSON != "" {
		var registryCredentials RegistryCredentials
		err := json.Unmarshal([]byte(registryCredentialsJSON), &registryCredentials)
		if err != nil {
			return fmt.Errorf("could not parse registry credentials: %w", err)
		}
		if err := registryCredentials.Validate(); err != nil {
			return fmt.Errorf("registryCredentials is not valid: %w", err)
		}
		c.Options.RegistryCredentials = registryCredentials
		logrus.Infof("loaded %d registry credentials", len(registryCredentials))
	}
	return nil
}

// CheckForErrors checks to make sure the configuration is valid
func (c Configuration) CheckForErrors() error {
	if c.Options.Organization == "" {
		return errors.New("options.organization not set")
	}
	return nil
}
