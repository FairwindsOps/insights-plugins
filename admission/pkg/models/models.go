package models

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
	Contents []byte
}
