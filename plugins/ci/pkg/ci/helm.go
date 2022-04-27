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
	return doHandleRemoteHelmChart(helm.Name, helm.Repo, chartName, helmRelease.Spec.Chart.Spec.Version, helm.ValuesFile, helm.ValuesFiles, helm.Values, helmRelease.Spec.Values, tempFolder, configFolder)
}

func handleRemoteHelmChart(helm models.HelmConfig, tempFolder string, configFolder string) error {
	if helm.Name == "" || helm.Chart == "" || helm.Repo == "" {
		return errors.New("Parameters 'name', 'repo' and 'chart' are required in helm definition")
	}
	return doHandleRemoteHelmChart(helm.Name, helm.Repo, helm.Chart, helm.Version, helm.ValuesFile, helm.ValuesFiles, helm.Values, nil, tempFolder, configFolder)
}

func doHandleRemoteHelmChart(helmName, repoURL, chartName, chartVersion, valuesFile string, valuesFiles []string, values, fluxValues map[string]interface{}, tempFolder, configFolder string) error {
	repoName := fmt.Sprintf("%s-%s-repo", helmName, chartName)
	_, err := commands.ExecWithMessage(exec.Command("helm", "repo", "add", repoName, repoURL), "Adding chart repository: "+repoName)
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

	helmValuesFileArgs, err := resolveHelmValuesToArgs(valuesFile, valuesFiles, values, fluxValues, tempFolder)
	if err != nil {
		return err
	}
	return doHandleLocalHelmChart(helmName, chartDownloadPath, helmValuesFileArgs, tempFolder, configFolder)
}

func handleLocalHelmChart(helm models.HelmConfig, baseRepoFolder, tempFolder string, configFolder string) error {
	if helm.Name == "" || helm.Path == "" {
		return errors.New("Parameters 'name' and 'path' are required in helm definition")
	}

	helmValuesFileArgs, err := resolveHelmValuesToArgs(helm.ValuesFile, helm.ValuesFiles, helm.Values, nil, tempFolder)
	if err != nil {
		return err
	}
	return doHandleLocalHelmChart(helm.Name, filepath.Join(baseRepoFolder, helm.Path), helmValuesFileArgs, tempFolder, configFolder)
}

func doHandleLocalHelmChart(helmName, helmPath string, helmValuesFileArgs []string, tempFolder, configFolder string) error {
	_, err := commands.ExecWithMessage(exec.Command("helm", "dependency", "update", helmPath), "Updating dependencies for "+helmName)
	if err != nil {
		return err
	}

	params := append([]string{"template", helmName, helmPath, "--output-dir", configFolder + helmName}, helmValuesFileArgs...)
	_, err = commands.ExecWithMessage(exec.Command("helm", params...), "Templating: "+helmName)
	if err != nil {
		return err
	}
	return nil
}

// resolveHelmValuesToArgs returns Helm CLI arguments of the form ["-f",
// "file1.yaml, ...] for Helm to process values from values files and the
// `values` keys specified in fairwinds-insights.yaml. The literal values are
// written to a file and specified last in the `-f ...` Helm arguments to
// ensure those files are applied last.
func resolveHelmValuesToArgs(valuesFile string, valuesFiles []string, values map[string]interface{}, fluxValues map[string]interface{}, tempFolder string) (helmArgs []string, err error) {
	hasValuesFile := valuesFile != ""
	hasValuesFiles := len(valuesFiles) > 0
	hasValues := len(values) > 0
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
		helmArgs = append(helmArgs, "-f", fluxValuesFilePath)
	}
	if hasValuesFile {
		helmArgs = append(helmArgs, "-f", filepath.Join(baseRepoFolder, valuesFile))
	}
	if hasValuesFiles {
		for _, i := range valuesFiles {
			helmArgs = append(helmArgs, "-f", filepath.Join(baseRepoFolder, i))
		}
	}
	if hasValues {
		yaml, err := yaml.Marshal(values)
		if err != nil {
			return nil, err
		}
		inlineValuesFilePath := tempFolder + "helm-values.yaml"
		err = ioutil.WriteFile(inlineValuesFilePath, yaml, 0644)
		if err != nil {
			return nil, err
		}
		helmArgs = append(helmArgs, "-f", inlineValuesFilePath)
	}
	return helmArgs, nil
}

// CopyYaml adds all Yaml found in a given spot into the manifest folder.
func (ci *CIScan) CopyYaml() error {
	for _, yamlPath := range ci.config.Manifests.YamlPaths {
		_, err := commands.ExecWithMessage(exec.Command("cp", "-r", filepath.Join(ci.repoBaseFolder, yamlPath), ci.configFolder), "Copying yaml file to config folder")
		if err != nil {
			return err
		}
	}
	return nil
}
