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
	Name         string            `yaml:"name"`
	Path         string            `yaml:"path"`
	VariableFile string            `yaml:"variableFile"`
	Variables    map[string]string `yaml:"variables"`
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

// GetDefaultConfig returns the default set of configuration options
func GetDefaultConfig() Configuration {
	return Configuration{
		Images: imageConfig{
			FolderName: "./insights/images",
		},
		Manifests: ManifestConfig{},
		Options: optionConfig{
			NewActionItemThreshold: 5,
			SeverityThreshold:      "danger",
			TempFolder:             "./insights/temp",
		},
	}
}

func maybeAddSlash(input string) string {
	if strings.HasSuffix(input, "/") {
		return input
	}
	return input + "/"
}

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
	c.Options.TempFolder = maybeAddSlash(c.Options.TempFolder)
	c.Images.FolderName = maybeAddSlash(c.Images.FolderName)
}

func (c Configuration) CheckForErrors() error {
	if c.Options.RepositoryName == "" {
		return errors.New("options.repositoryName not set")
	}
	if c.Options.Organization == "" {
		return errors.New("options.organization not set")
	}
	return nil
}
