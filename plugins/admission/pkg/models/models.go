package models

import (
	"github.com/fairwindsops/insights-plugins/plugins/opa/pkg/opa"
	polarisconfiguration "github.com/fairwindsops/polaris/pkg/config"
)

// ScoreOutOfBoundsMessage is the message for the error when the score returned by Insights is out of bounds.
const ScoreOutOfBoundsMessage = "score out of bounds"

// Resource represents a Kubernetes resource with information about what file it came from.
type Resource struct {
	Kind      string
	Name      string
	Filename  string
	HelmName  string
	Namespace string
}

// ReportInfo is the information about a run of one of the reports.
type ReportInfo struct {
	Report   string
	Version  string
	Contents []byte
}

// Configuration saves any config from Insights.
type Configuration struct {
	Reports struct {
		Polaris bool
		Pluto   bool
		OPA     bool
	}
	OPA struct {
		CustomChecks         []opa.OPACustomCheck // contains both checks and libraries (IsLibrary)
		CustomCheckInstances []opa.CheckSetting
	}
	Polaris *polarisconfiguration.Configuration
}

type InsightsConfig struct {
	Hostname        string
	Organization    string
	Cluster         string
	Token           string
	IgnoreUsernames []string
}
