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
	Images    folderConfig   `yaml:"images"`
	Manifests ManifestConfig `yaml:"manifests"`
	Options   optionConfig   `yaml:"options"`
}

// ManifestConfig is a struct representing the config options for Manifests
type ManifestConfig struct {
	FolderName string       `yaml:"folder"`
	Helm       []HelmConfig `yaml:"helm"`
}

// HelmConfig is the configuration for helm.
type HelmConfig struct {
	Name         string `yaml:"name"`
	Path         string `yaml:"path"`
	VariableFile string `yaml:"variables"`
}

type optionConfig struct {
	Fail                 bool    `yaml:"fail"`
	ScoreThreshold       float64 `yaml:"scoreThreshold"`
	ScoreChangeThreshold float64 `yaml:"scoreChangeThreshold"`
	TempFolder           string  `yaml:"tempFolder"`
	Hostname             string  `yaml:"hostname"`
	Organization         string  `yaml:"organization"`
	JUnitOutput          string  `yaml:"junitOutput"`
	RepositoryName       string  `yaml:"repositoryName"`
}

type folderConfig struct {
	FolderName string   `yaml:"folder"`
	Commands   []string `yaml:"cmd"`
}

// ScanResults is the value returned by the Insights API upon submitting a scan.
type ScanResults struct {
	BaselineScore float64
	Score         float64
	ActionItems   []actionItem
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
		Images: folderConfig{
			FolderName: "./insights/images",
		},
		Manifests: ManifestConfig{
			FolderName: "./insights/manifests",
		},
		Options: optionConfig{
			ScoreThreshold:       0.6,
			ScoreChangeThreshold: 0.4,
			TempFolder:           "./insights/temp",
		},
	}
}
