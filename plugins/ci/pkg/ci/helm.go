package ci

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/commands"
	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/models"
	"github.com/ghodss/yaml"
	"github.com/sirupsen/logrus"
)

// ProcessHelmTemplates turns helm into yaml to be processed by Polaris or the other tools.
func (ci *CIScan) ProcessHelmTemplates() error {
	for _, helm := range ci.config.Manifests.Helm {
		if helm.IsLocal() && helm.IsRemote() {
			return fmt.Errorf("Error in helm definition %v - It is not possible to use both 'path' and 'repo' simultaneously", helm.Name)
		}
		if helm.IsLocal() {
			err := handleLocalHelmChart(helm, ci.repoBaseFolder, ci.config.Options.TempFolder, ci.configFolder)
			if err != nil {
				return err
			}
		} else if helm.IsRemote() {
			if helm.IsFluxFile() {
				err := handleFluxHelmChart(helm, ci.repoBaseFolder, ci.config.Options.TempFolder, ci.configFolder)
				if err != nil {
					return err
				}
			} else {
				err := handleRemoteHelmChart(helm, ci.config.Options.TempFolder, ci.configFolder)
				if err != nil {
					return err
				}
			}
		} else {
			logrus.Debugf("cannot determine the type of helm config for: %#v\n", helm)
			return fmt.Errorf("Could not determine the type of helm config.: %v", helm.Name)
		}
	}
	return nil
}

func handleFluxHelmChart(helm models.HelmConfig, baseRepoFolder, tempFolder string, configFolder string) error {
	if helm.Name == "" || helm.Repo == "" {
		return errors.New("Parameters 'name', 'repo' are required when using fluxFile")
	}

	fluxFile, err := os.Open(filepath.Join(baseRepoFolder, helm.FluxFile))
	if err != nil {
		return fmt.Errorf("Unable to open file %v: %v", helm.FluxFile, err)
	}

	fluxFileContent, err := ioutil.ReadAll(fluxFile)
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
	_, err := commands.ExecWithMessage(exec.Command("helm", "repo", "add", repoName, helm.Repo), "Adding chart repository: "+repoName)
	if err != nil {
		return err
	}

	repoDownloadPath := fmt.Sprintf("%s/downloaded-charts/%s/", tempFolder, repoName)
	chartDownloadPath := repoDownloadPath + chartName

	chartFullName := fmt.Sprintf("%s/%s", repoName, chartName)
	params := []string{"fetch", chartFullName, "--untar", "--destination", repoDownloadPath}

	if chartVersion != "" {
		params = append(params, "--version", chartVersion)
	} else {
		logrus.Infof("version for chart %v not found, using latest...", chartFullName)
	}
	_, err = commands.ExecWithMessage(exec.Command("helm", params...), fmt.Sprintf("Retrieving pkg %v from repository %v, downloading it locally and unziping it", chartName, repoName))
	if err != nil {
		return err
	}

	helmValuesFiles, err := processHelmValues(helm, fluxValues, tempFolder)
	if err != nil {
		return err
	}
	return doHandleLocalHelmChart(helm, "", chartDownloadPath, helmValuesFiles, tempFolder, configFolder)
}

func handleLocalHelmChart(helm models.HelmConfig, baseRepoFolder, tempFolder string, configFolder string) error {
	if helm.Name == "" || helm.Path == "" {
		return errors.New("Parameters 'name' and 'path' are required in helm definition")
	}

	helmValuesFiles, err := processHelmValues(helm, nil, tempFolder)
	if err != nil {
		return err
	}
	return doHandleLocalHelmChart(helm, baseRepoFolder, helm.Path, helmValuesFiles, tempFolder, configFolder)
}

func doHandleLocalHelmChart(helm models.HelmConfig, repoPath string, helmPath string, helmValuesFiles []string, tempFolder, configFolder string) error {
	helmPath = filepath.Join(repoPath, helmPath)
	_, err := commands.ExecWithMessage(exec.Command("helm", "dependency", "update", helmPath), "Updating dependencies for "+helm.Name)
	if err != nil {
		return err
	}

	var helmValuesFileArgs []string
	for _, vf := range helmValuesFiles {
		helmValuesFileArgs = append(helmValuesFileArgs, "-f", filepath.Join(repoPath, vf))
	}
	params := append([]string{"template", helm.Name, helmPath, "--output-dir", configFolder + helm.Name}, helmValuesFileArgs...)
	_, err = commands.ExecWithMessage(exec.Command("helm", params...), "Templating: "+helm.Name)
	if err != nil {
		return err
	}
	return nil
}

// processHelmValues returns a slice of HElm values files after processing values and values-files from a models.HelmConfig and the
// fluxValues parameter. Any Helm values are written to a file.
func processHelmValues(helm models.HelmConfig, fluxValues map[string]interface{}, tempFolder string) (valuesFileNames []string, err error) {
	hasValuesFile := helm.ValuesFile != ""
	hasValuesFiles := len(helm.ValuesFiles) > 0
	hasValues := len(helm.Values) > 0
	hasFluxValues := len(fluxValues) > 0

	if hasFluxValues {
		yaml, err := yaml.Marshal(fluxValues)
		if err != nil {
			return nil, err
		}
		fluxValuesFilePath := tempFolder + "flux-helm-values.yaml"
		err = ioutil.WriteFile(fluxValuesFilePath, yaml, 0644)
		if err != nil {
			return nil, err
		}
		valuesFileNames = append(valuesFileNames, fluxValuesFilePath)
	}
	if hasValuesFile {
		valuesFileNames = append(valuesFileNames, helm.ValuesFile)
	}
	if hasValuesFiles {
		if hasValuesFile {
			logrus.Warnf("Both ValuesFile and ValuesFiles are present in Helm configuration %q, it is recommended to list all values files in the ValuesFiles list", helm.Name)
		}
		for _, i := range helm.ValuesFiles {
			valuesFileNames = append(valuesFileNames, i)
		}
	}
	if hasValues {
		yaml, err := yaml.Marshal(helm.Values)
		if err != nil {
			return nil, err
		}
		inlineValuesFilePath := tempFolder + "fairwinds-insights-helm-values.yaml"
		err = ioutil.WriteFile(inlineValuesFilePath, yaml, 0644)
		if err != nil {
			return nil, err
		}
		valuesFileNames = append(valuesFileNames, inlineValuesFilePath)
	}
	logrus.Debugf("returning processed Helm values and values-files as these Helm values file names: %v for Helm configuration: %#v\n", valuesFileNames, helm)
	return valuesFileNames, nil
}

// CopyYaml adds all Yaml found in a given spot into the manifest folder.
func (ci *CIScan) CopyYaml() error {
	for _, yamlPath := range ci.config.Manifests.YamlPaths {
		_, err := commands.ExecWithMessage(exec.Command("cp", "-r", filepath.Join(ci.repoBaseFolder, yamlPath), ci.configFolder), "Copying yaml file to config folder: " + yamlPath)
		if err != nil {
			return err
		}
	}
	return nil
}
