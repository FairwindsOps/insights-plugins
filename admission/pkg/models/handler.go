package models

import (
	"github.com/fairwindsops/insights-plugins/opa/pkg/opa"
	polarisconfiguration "github.com/fairwindsops/polaris/pkg/config"
)

// Configuration saves any config from Insights.
type Configuration struct {
	Reports struct {
		Polaris bool
		Pluto   bool
		OPA     bool
	}
	OPA struct {
		CustomChecks         []opa.OPACustomCheck
		CustomCheckInstances []opa.CheckSetting
	}
	Polaris *polarisconfiguration.Configuration
}
