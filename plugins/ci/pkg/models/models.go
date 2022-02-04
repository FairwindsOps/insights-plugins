package models

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ScoreOutOfBoundsMessage is the message for the error when the score returned by Insights is out of bounds.
const ScoreOutOfBoundsMessage = "score out of bounds"

// Resource represents a Kubernetes resource with information about what file it came from.
type Resource struct {
	Kind       string
	Name       string
	Filename   string
	Namespace  string
	HelmName   string
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
	Images    imageConfig    `yaml:"images"`
	Manifests ManifestConfig `yaml:"manifests"`
	Options   optionConfig   `yaml:"options"`
	Reports   reportsConfig  `yaml:"reports"`
}

// ManifestConfig is a struct representing the config options for Manifests
type ManifestConfig struct {
	YamlPaths []string     `yaml:"yaml"`
	Helm      []HelmConfig `yaml:"helm"`
}

// HelmConfig is the configuration for helm.
type HelmConfig struct {
	Name       string                 `yaml:"name"`
	Path       string                 `yaml:"path"`
	Repo       string                 `yaml:"repo"`
	Chart      string                 `yaml:"chart"`
	FluxFile   string                 `yaml:"fluxFile"`
	Version    string                 `yaml:"version"`
	ValuesFile string                 `yaml:"valuesFile"`
	Values     map[string]interface{} `yaml:"values"`
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
	Polaris reportConfig `yaml:"polaris"`
	Pluto   reportConfig `yaml:"pluto"`
	Trivy   trivyConfig  `yaml:"trivy"`
	OPA     reportConfig `yaml:"opa"`
}

type reportConfig struct {
	Enabled *bool `yaml:"enabled"`
}

type trivyConfig struct {
	Enabled       *bool `yaml:"enabled"`
	SkipManifests *bool `yaml:"skipManifests"`
}

type optionConfig struct {
	SetExitCode            bool   `yaml:"setExitCode"`
	BaseBranch             string `yaml:"baseBranch"`
	NewActionItemThreshold int    `yaml:"newActionItemThreshold"`
	SeverityThreshold      string `yaml:"severityThreshold"`
	TempFolder             string `yaml:"tempFolder"`
	Hostname               string `yaml:"hostname"`
	Organization           string `yaml:"organization"`
	JUnitOutput            string `yaml:"junitOutput"`
	RepositoryName         string `yaml:"repositoryName"`
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

	// TODO: images are copied via script insights-ci.sh, what now?
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
// it should follow the order:
// - file content > env. variables > default
func (c *Configuration) SetDefaults() {
	if c.Options.BaseBranch == "" {
		c.Options.BaseBranch = "master"
	}
	if c.Options.Organization == "" {
		orgName := strings.TrimSpace(os.Getenv("ORG_NAME"))
		if orgName != "" {
			c.Options.Organization = orgName
		}
	}
	if c.Options.RepositoryName == "" {
		repoName := strings.TrimSpace(os.Getenv("REPOSITORY_NAME"))
		if repoName != "" {
			c.Options.RepositoryName = repoName
		}
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
}

// CheckForErrors checks to make sure the configuration is valid
func (c Configuration) CheckForErrors() error {
	if c.Options.Organization == "" {
		return errors.New("options.organization not set")
	}
	return nil
}
