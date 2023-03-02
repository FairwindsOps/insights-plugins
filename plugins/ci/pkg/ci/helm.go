package ci

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/commands"
	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/models"
	"github.com/ghodss/yaml"
	"github.com/hashicorp/go-multierror"
	"github.com/sirupsen/logrus"
)

// ProcessHelmTemplates turns helm into yaml to be processed by Polaris or the other tools.
func (ci *CIScan) ProcessHelmTemplates() error {
	var allErrs *multierror.Error = new(multierror.Error)
	for _, helm := range ci.config.Manifests.Helm {
		if helm.IsLocal() && helm.IsRemote() {
			allErrs = multierror.Append(allErrs, fmt.Errorf("Error in helm definition %v - It is not possible to use both 'path' and 'repo' simultaneously", helm.Name))
		}
		if helm.IsLocal() {
			err := handleLocalHelmChart(helm, ci.repoBaseFolder, ci.config.Options.TempFolder, ci.configFolder)
			if err != nil {
				allErrs = multierror.Append(allErrs, err)
			}
		} else if helm.IsRemote() {
			if helm.IsFluxFile() {
				err := handleFluxHelmChart(helm, ci.repoBaseFolder, ci.config.Options.TempFolder, ci.configFolder)
				if err != nil {
					allErrs = multierror.Append(allErrs, err)
				}
			} else {
				err := handleRemoteHelmChart(helm, ci.config.Options.TempFolder, ci.configFolder)
				if err != nil {
					allErrs = multierror.Append(allErrs, err)
				}
			}
		} else {
			logrus.Debugf("cannot determine the type of helm config for: %#v\n", helm)
			allErrs = multierror.Append(allErrs, fmt.Errorf("Could not determine the type of helm config.: %v", helm.Name))
		}
	}
	return allErrs.ErrorOrNil()
}

func handleFluxHelmChart(helm models.HelmConfig, baseRepoFolder, tempFolder string, configFolder string) error {
	if helm.Name == "" || helm.Repo == "" {
		return errors.New("Parameters 'name', 'repo' are required when using fluxFile")
	}

	fluxFile, err := os.Open(filepath.Join(baseRepoFolder, helm.FluxFile))
	if err != nil {
		return fmt.Errorf("Unable to open file %v: %v", helm.FluxFile, err)
	}

	fluxFileContent, err := io.ReadAll(fluxFile)
	if err != nil {
		return fmt.Errorf("Unable to read file %v: %v", helm.FluxFile, err)
	}

	// Ideally we should use https://fluxcd.io/docs/components/helm/api/#helm.toolkit.fluxcd.io%2fv1
	// However, the attempt was not possible due to being unable to parse `v1.Duration` successfully
	type HelmReleaseModel struct {
		Spec struct {
			Chart struct {
				Spec struct {
					Chart   string `yaml:"chart"`
					Version string `yaml:"version"`
				} `yaml:"spec"`
			} `yaml:"chart"`
			Values     map[string]interface{} `yaml:"values"`
			ValuesFrom []interface{}          `yaml:"valuesFrom"`
		} `yaml:"spec"`
	}

	var helmRelease HelmReleaseModel
	err = yaml.Unmarshal(fluxFileContent, &helmRelease)
	if err != nil {
		return fmt.Errorf("Unable to unmarshal file %v: %v", helm.FluxFile, err)
	}

	chartName := helmRelease.Spec.Chart.Spec.Chart
	if chartName == "" {
		return fmt.Errorf("Could not find required spec.chart.spec.chart in fluxFile %v", helm.FluxFile)
	}

	if len(helmRelease.Spec.ValuesFrom) > 0 {
		logrus.Warnf("fluxFile: %v - spec.valuesFrom not supported, it won't be applied...", helm.FluxFile)
	}
	return doHandleRemoteHelmChart(helm, chartName, helmRelease.Spec.Chart.Spec.Version, helmRelease.Spec.Values, tempFolder, configFolder)
}

func handleRemoteHelmChart(helm models.HelmConfig, tempFolder string, configFolder string) error {
	if helm.Name == "" || helm.Chart == "" || helm.Repo == "" {
		return errors.New("Parameters 'name', 'repo' and 'chart' are required in helm definition")
	}
	return doHandleRemoteHelmChart(helm, helm.Chart, helm.Version, nil, tempFolder, configFolder)
}

func doHandleRemoteHelmChart(helm models.HelmConfig, chartName, chartVersion string, fluxValues map[string]interface{}, tempFolder, configFolder string) error {
	repoName := fmt.Sprintf("%s-%s-repo", helm.Name, chartName)
	output, err := commands.ExecWithMessage(exec.Command("helm", "repo", "add", repoName, helm.Repo), "Adding chart repository: "+repoName)
	if err != nil {
		return models.ScanErrorsReportResult{
			ErrorMessage: fmt.Sprintf("%v: %s", err, output),
			ErrorContext: fmt.Sprintf("adding helm repository %q", helm.Repo),
			Kind:         "HelmChart",
			ResourceName: helm.Name,
			Filename:     helm.Name,
			Remediation:  "Verify that the helm repository is valid.",
		}
	}

	repoDownloadPath := fmt.Sprintf("%s/downloaded-charts/%s/", tempFolder, repoName)
	chartDownloadPath := repoDownloadPath + chartName

	chartFullName := fmt.Sprintf("%s/%s", repoName, chartName)
	params := []string{"fetch", chartFullName, "--untar", "--destination", repoDownloadPath}

	var versionDisplay string
	if chartVersion != "" {
		params = append(params, "--version", chartVersion)
		versionDisplay = fmt.Sprintf("version %s", chartVersion)
	} else {
		logrus.Infof("version for chart %v not found, using latest...", chartFullName)
		versionDisplay = "the latest version"
	}
	output, err = commands.ExecWithMessage(exec.Command("helm", params...), fmt.Sprintf("Retrieving %s of pkg %v from repository %v, downloading it locally and unziping it", versionDisplay, chartName, repoName))
	if err != nil {
		return models.ScanErrorsReportResult{
			ErrorMessage: fmt.Sprintf("%v: %s", err, output),
			ErrorContext: fmt.Sprintf("fetching %s of the helm chart %s from %s", versionDisplay, chartName, repoName),
			Kind:         "HelmChart",
			ResourceName: helm.Name,
			Filename:     helm.Name,
			Remediation:  "Verify that this version of the helm chart is available in the helm repository.",
		}
	}

	helmValuesFiles, err := processHelmValues(helm, fluxValues, tempFolder)
	if err != nil {
		return models.ScanErrorsReportResult{
			ErrorMessage: err.Error(),
			ErrorContext: "processing helm values files",
			Kind:         "HelmChart",
			ResourceName: helm.Name,
			Filename:     helm.Name,
		}
	}
	return doHandleLocalHelmChart(helm, "", chartDownloadPath, helmValuesFiles, tempFolder, configFolder)
}

func handleLocalHelmChart(helm models.HelmConfig, baseRepoFolder, tempFolder string, configFolder string) error {
	if helm.Name == "" || helm.Path == "" {
		return errors.New("Parameters 'name' and 'path' are required in helm definition")
	}

	helmValuesFiles, err := processHelmValues(helm, nil, tempFolder)
	if err != nil {
		return models.ScanErrorsReportResult{
			ErrorMessage: err.Error(),
			ErrorContext: "processing helm values files",
			Kind:         "HelmChart",
			ResourceName: helm.Name,
			Filename:     helm.Name,
		}
	}
	return doHandleLocalHelmChart(helm, baseRepoFolder, helm.Path, helmValuesFiles, tempFolder, configFolder)
}

func doHandleLocalHelmChart(helm models.HelmConfig, repoPath string, helmPath string, helmValuesFiles []helmValuesFile, tempFolder, configFolder string) error {
	fullHelmPath := filepath.Join(repoPath, helmPath)
	cleanHelmPath := filepath.Clean(helmPath) // Remove things like `./`
	output, err := commands.ExecWithMessage(exec.Command("helm", "dependency", "update", fullHelmPath), "Updating dependencies for "+helm.Name)
	if err != nil {
		return models.ScanErrorsReportResult{
			ErrorMessage: fmt.Sprintf("%v: %s", err, output),
			ErrorContext: fmt.Sprintf("updating dependencies for helm chart %s", helm.Name),
			Kind:         "HelmChart",
			ResourceName: helm.Name,
			Filename:     cleanHelmPath,
			Remediation:  "Examine the Chart.yaml file for errors in the dependency specification, or invalid dependencies. See also, https://helm.sh/docs/helm/helm_dependency/",
		}
	}

	var helmValuesFileArgs []string
	for _, vf := range helmValuesFiles {
		if vf.tmp {
			helmValuesFileArgs = append(helmValuesFileArgs, "-f", vf.path)
		} else {
			helmValuesFileArgs = append(helmValuesFileArgs, "-f", filepath.Join(repoPath, vf.path))
		}
	}
	params := append([]string{"template", helm.Name, fullHelmPath, "--output-dir", configFolder + helm.Name}, helmValuesFileArgs...)
	output, err = commands.ExecWithMessage(exec.Command("helm", params...), "Templating: "+helm.Name)
	if err != nil {
		return models.ScanErrorsReportResult{
			ErrorMessage: fmt.Sprintf("%v: %s", err, output),
			ErrorContext: fmt.Sprintf("templating helm chart %s", helm.Name),
			Kind:         "HelmChart",
			ResourceName: helm.Name,
			Filename:     cleanHelmPath,
			Remediation:  "Examine the Helm template, values files, and any inline values that may be specified in fairwinds-insights.yaml, for syntax errors that cause the `helm template` command to fail.",
		}
	}
	return nil
}

type helmValuesFile struct {
	path string
	tmp  bool // tmp files are files created from inline values definition
}

// processHelmValues returns a slice of HElm values files after processing values and values-files from a models.HelmConfig and the
// fluxValues parameter. Any Helm values are written to a file.
func processHelmValues(helm models.HelmConfig, fluxValues map[string]interface{}, tempFolder string) (valuesFiles []helmValuesFile, err error) {
	hasValuesFile := helm.ValuesFile != ""
	hasValuesFiles := len(helm.ValuesFiles) > 0
	hasValues := len(helm.Values) > 0
	hasFluxValues := len(fluxValues) > 0

	if hasFluxValues {
		yaml, err := yaml.Marshal(fluxValues)
		if err != nil {
			return nil, err
		}
		fluxValuesFilePath := filepath.Join(tempFolder, "flux-helm-values.yaml")
		err = os.WriteFile(fluxValuesFilePath, yaml, 0644)
		if err != nil {
			return nil, err
		}
		valuesFiles = append(valuesFiles, helmValuesFile{path: fluxValuesFilePath, tmp: true})
	}
	if hasValuesFile {
		valuesFiles = append(valuesFiles, helmValuesFile{path: helm.ValuesFile})
	}
	if hasValuesFiles {
		if hasValuesFile {
			logrus.Warnf("Both ValuesFile and ValuesFiles are present in Helm configuration %q, it is recommended to list all values files in the ValuesFiles list", helm.Name)
		}
		for _, vf := range helm.ValuesFiles {
			valuesFiles = append(valuesFiles, helmValuesFile{path: vf})
		}
	}
	if hasValues {
		yaml, err := yaml.Marshal(helm.Values)
		if err != nil {
			return nil, err
		}
		inlineValuesFilePath := filepath.Join(tempFolder, "fairwinds-insights-helm-values.yaml")
		err = os.WriteFile(inlineValuesFilePath, yaml, 0644)
		if err != nil {
			return nil, err
		}
		logrus.Infof("added %s to valuesFiles", inlineValuesFilePath)
		valuesFiles = append(valuesFiles, helmValuesFile{path: inlineValuesFilePath, tmp: true})
	}
	logrus.Debugf("returning processed Helm values and values-files as these Helm values file names: %v for Helm configuration: %#v\n", valuesFiles, helm)
	return valuesFiles, nil
}

// CopyYaml adds all Yaml found in a given spot into the manifest folder.
func (ci *CIScan) CopyYaml() error {
	var numFailures int64
	for _, yamlPath := range ci.config.Manifests.YamlPaths {
		destFolder, err := createDestinationFolderIfNotExists(ci.configFolder, yamlPath)
		if err != nil {
			numFailures++
		}
		_, err = commands.ExecWithMessage(exec.Command("cp", "-r", filepath.Join(ci.repoBaseFolder, yamlPath), destFolder), "Copying yaml file to config folder: "+yamlPath)
		if err != nil {
			numFailures++
			// The error is already logged by commands.ExecWithMessage
		}
	}
	if numFailures == 0 {
		return nil
	}
	return fmt.Errorf("%d of %d yaml files failed to be copied to the config directory, see logs for the individual error messages", numFailures, len(ci.config.Manifests.YamlPaths))
}

func createDestinationFolderIfNotExists(configFolder, relYamlPath string) (string, error) {
	// from file ./specific_folder/file1.yaml should create the dir {configFolder}/specific_folder if not yet exists
	relFileDir, _ := filepath.Split(relYamlPath)
	targetFolder := filepath.Join(configFolder, relFileDir)
	_, err := commands.ExecWithMessage(exec.Command("mkdir", "-p", targetFolder), "creating destination folder: "+targetFolder)
	return targetFolder, err
}
