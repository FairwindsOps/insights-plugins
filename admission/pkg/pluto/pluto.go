package pluto

import (
	"github.com/fairwindsops/pluto/v3/pkg/api"

	"github.com/fairwindsops/insights-plugins/admission/pkg/models"
)

func ProcessPluto(input []byte) (models.ReportInfo, error) {
	report := models.ReportInfo{
		Report: "pluto",
	}
	_, _, err := api.GetDefaultVersionList()
	if err != nil {
		return report, err
	}
	/*
		apiInstance := &api.Instance{
			TargetVersions:     targetVersions,
			OutputFormat:       outputFormat,
			CustomColumns:      customColumns,
			IgnoreDeprecations: ignoreDeprecations,
			IgnoreRemovals:     ignoreRemovals,
			OnlyShowRemoved:    onlyShowRemoved,
			DeprecatedVersions: deprecatedVersionList,
			Components:         componentList,
		}*/

	return report, nil
}
