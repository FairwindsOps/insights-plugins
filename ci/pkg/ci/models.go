package ci

// ScoreOutOfBoundsMessage is the message for the error when the score returned by Insights is out of bounds.
const ScoreOutOfBoundsMessage = "score out of bounds"

// Configuration is a struct representing the config options for Insights CI/CD
type Configuration struct {
	Images    folderConfig `yaml:"images"`
	Manifests folderConfig `yaml:"manifests"`
	Options   optionConfig `yaml:"options"`
}

type optionConfig struct {
	Fail                 bool    `yaml:"fail"`
	ScoreThreshold       float64 `yaml:"scoreThreshold"`
	ScoreChangeThreshold float64 `yaml:"scoreChangeThreshold"`
	TempFolder           string  `yaml:"tempFolder"`
	Hostname             string  `yaml:"hostname"`
	Organization         string  `yaml:"organization"`
	JUnitOutput          string  `yaml:"junitOutput"`
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
	Description  string
}

// GetDefaultConfig returns the default set of configuration options
func GetDefaultConfig() Configuration {
	return Configuration{
		Images: folderConfig{
			FolderName: "./insights/images",
		},
		Manifests: folderConfig{
			FolderName: "./insights/manifests",
		},
		Options: optionConfig{
			ScoreThreshold:       0.6,
			ScoreChangeThreshold: 0.4,
			TempFolder:           "./insights/temp",
		},
	}
}
