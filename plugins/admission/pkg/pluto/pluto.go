package pluto

import (
	"encoding/json"

	"github.com/fairwindsops/pluto/v3/pkg/api"

	"github.com/fairwindsops/insights-plugins/plugins/admission/pkg/models"
)

const plutoVersion = "3.4.1"

// ProcessPluto processes an object with Pluto.
func ProcessPluto(input []byte) (models.ReportInfo, error) {
	report := models.ReportInfo{
		Report:  "pluto",
		Version: plutoVersion,
	}
	deprecatedVersionList, targetVersions, err := api.GetDefaultVersionList()
	if err != nil {
		return report, err
	}
	var componentList []string
	for _, v := range deprecatedVersionList {
		if !api.StringInSlice(v.Component, componentList) {
			// if the pass-in components are nil, then we use the versions in the main list
			componentList = append(componentList, v.Component)
		}
	}

	apiInstance := &api.Instance{
		TargetVersions:     targetVersions,
		OutputFormat:       "json",
		IgnoreDeprecations: false,
		IgnoreRemovals:     false,
		OnlyShowRemoved:    false,
		DeprecatedVersions: deprecatedVersionList,
		Components:         componentList,
	}

	apiInstance.Outputs, err = apiInstance.IsVersioned(input)
	if err != nil {
		return report, err
	}
	report.Contents, err = json.Marshal(apiInstance)
	if err != nil {
		return report, err
	}
	return report, nil
}
