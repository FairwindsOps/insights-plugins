package ci

import (
	"errors"
	"strings"
)

// ScoreOutOfBoundsMessage is the message for the error when the score returned by Insights is out of bounds.
const ScoreOutOfBoundsMessage = "score out of bounds"

// Resource represents a Kubernetes resource with information about what file it came from.
type Resource struct {
	Kind      string
	Name      string
	Filename  string
	Namespace string
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
	ValuesFile string                 `yaml:"valuesFile"`
	Values     map[string]interface{} `yaml:"values"`
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
	NewActionItems   []actionItem
	FixedActionItems []actionItem
	Pass             bool
}

type actionItem struct {
	Remediation  string
	Severity     float64
	Title        string
	ResourceName string
	ResourceKind string
	Description  string
	Notes        string
}

func maybeAddSlash(input string) string {
	if strings.HasSuffix(input, "/") {
		return input
	}
	return input + "/"
}

// SetDefaults sets configurationd defaults
func (c *Configuration) SetDefaults() {
	if c.Options.TempFolder == "" {
		c.Options.TempFolder = "./_insightsTemp/"
	}
	if c.Images.FolderName == "" {
		c.Images.FolderName = "./_insightsTempImages/"
	}
	if c.Options.BaseBranch == "" {
		c.Options.BaseBranch = "master"
	}
	if c.Options.Hostname == "" {
		c.Options.Hostname = "https://insights.fairwinds.com"
	}
	if c.Options.SeverityThreshold == "" {
		c.Options.SeverityThreshold = "danger"
	}
	c.Options.TempFolder = maybeAddSlash(c.Options.TempFolder)
	c.Images.FolderName = maybeAddSlash(c.Images.FolderName)
}

// CheckForErrors checks to make sure the configuration is valid
func (c Configuration) CheckForErrors() error {
	if c.Options.RepositoryName == "" {
		return errors.New("options.repositoryName not set")
	}
	if c.Options.Organization == "" {
		return errors.New("options.organization not set")
	}
	return nil
}
