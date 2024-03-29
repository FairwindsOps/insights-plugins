package models

// TFSecResult contains a single TFSec finding.
type TFSecResult struct {
	RuleID          string              `json:"rule_id"`
	RuleDescription string              `json:"rule_description"`
	Severity        string              `json:"severity"`
	Description     string              `json:"description"`
	Impact          string              `json:"impact"`
	Links           []string            `json:"links"`
	Resolution      string              `json:"resolution"`
	Resource        string              `json:"resource"` // TF resource E.G. aws_instance.bastion
	Location        TFSecResultLocation `json:"location"`
	LongID          string              `json:"long_id"`
}

// TFSecResultLocation contains the file name and line numbers where an issue
// was found.
type TFSecResultLocation struct {
	FileName  string `json:"filename"`
	StartLine int64  `json:"start_line"`
	EndLine   int64  `json:"end_line"`
}

// TFSecReportProperties contains multiple TFSec results.
type TFSecReportProperties struct {
	Items []TFSecResult `json:"results"`
}
