package pluto

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/rogpeppe/go-internal/semver"
	"github.com/sirupsen/logrus"

	plutoversionsfile "github.com/fairwindsops/pluto/v5"
	"github.com/fairwindsops/pluto/v5/pkg/api"

	"github.com/fairwindsops/insights-plugins/plugins/admission/pkg/models"
)

const plutoVersion = "5.8.0"

// ProcessPluto processes an object with Pluto.
func ProcessPluto(input []byte) (models.ReportInfo, error) {
	report := models.ReportInfo{
		Report:  "pluto",
		Version: plutoVersion,
	}
	deprecatedVersionList, targetVersions, err := api.GetDefaultVersionList(plutoversionsfile.Content())
	if err != nil {
		return report, err
	}
	userTargetVersionsStr := os.Getenv("PLUTO_TARGET_VERSIONS")
	userTargetVersions, err := parsePlutoTargetVersions(userTargetVersionsStr)
	if err != nil {
		logrus.Errorf("unable to parse pluto target versions %q: %w", userTargetVersionsStr, err)
		return report, err
	}
	if userTargetVersions != nil {
		logrus.Debugf("Updating pluto target versions with user-specified ones: %#v", userTargetVersions)
		for k, v := range userTargetVersions {
			targetVersions[k] = v
		}
	}
	logrus.Debugf("Using pluto target versions: %#v", targetVersions)
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
	_ = apiInstance.DisplayOutput()
	report.Contents, err = json.Marshal(apiInstance)
	if err != nil {
		return report, err
	}
	return report, nil
}

// parsePlutoTargetVersions converts a string of the form
// key=value[,key=value...] into a map[string]string, validating that the
// values are a valid semver version.
func parsePlutoTargetVersions(vs string) (vm map[string]string, err error) {
	if vs == "" {
		return nil, nil
	}
	var ss []string
	n := strings.Count(vs, "=")
	switch n {
	case 0:
		return nil, fmt.Errorf("please format %q as key=value", vs)
	case 1:
		ss = append(ss, strings.Trim(vs, `"`))
	default:
		r := csv.NewReader(strings.NewReader(vs))
		var err error
		ss, err = r.Read()
		if err != nil {
			return nil, fmt.Errorf("while parsing %q: %w", vs, err)
		}
	}
	vm = make(map[string]string, len(ss))
	for _, pair := range ss {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("please format %q as key=value", pair)
		}
		vm[kv[0]] = kv[1]
	}

	var invalidTargetVersions []string
	for k, v := range vm {
		if !semver.IsValid(v) {
			invalidTargetVersions = append(invalidTargetVersions, k)
			logrus.Errorf("Invalid version %q for target %s, please use a valid semver version with a leading 'v'", v, k)
		}
	}
	if len(invalidTargetVersions) > 0 {
		return nil, fmt.Errorf("specified target versions are not valid semver with a leading 'v' for: %#v", invalidTargetVersions)
	}
	return vm, nil
}
