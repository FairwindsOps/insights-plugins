package ci

// ScoreOutOfBoundsMessage is the message for the error when the score returned by Insights is out of bounds.
const ScoreOutOfBoundsMessage = "score out of bounds"

// Resource represents a Kubernetes resource with information about what file it came from.
type Resource struct {
	Kind        string
	Name        string
	Filename    string
	FileComment string
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
