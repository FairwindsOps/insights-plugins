package ci

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	trivymodels "github.com/fairwindsops/insights-plugins/trivy/pkg/models"
	"github.com/sirupsen/logrus"
	"github.com/thoas/go-funk"
	"gopkg.in/yaml.v3"

	"github.com/fairwindsops/insights-plugins/ci/pkg/commands"
	"github.com/fairwindsops/insights-plugins/ci/pkg/models"
	"github.com/fairwindsops/insights-plugins/ci/pkg/util"
)

type formFile struct {
	field    string
	filename string
	location string
}

type gitInfo struct {
	origin        string
	branch        string
	masterHash    string
	currentHash   string
	commitMessage string
	repoName      string
}

// GetAllResources scans a folder of yaml files and returns all of the images and resources used.
func GetAllResources(configDir string, configurationObject models.Configuration) ([]trivymodels.Image, []models.Resource, error) {
	images := make([]trivymodels.Image, 0)
	resources := make([]models.Resource, 0)
	err := filepath.Walk(configDir, func(path string, info os.FileInfo, err error) error {
		if !strings.HasSuffix(info.Name(), ".yaml") && !strings.HasSuffix(info.Name(), ".yml") {
			return nil
		}

		displayFilename, err := filepath.Rel(configDir, path)
		if err != nil {
			return err
		}
		var helmName string
		for _, helm := range configurationObject.Manifests.Helm {
			if strings.HasPrefix(displayFilename, helm.Name+"/") {
				parts := strings.Split(displayFilename, "/")
				parts = parts[2:]
				displayFilename = strings.Join(parts, "/")
				if helm.IsLocal() {
					displayFilename = filepath.Join(helm.Path, displayFilename)
				} else if helm.IsRemote() {
					if helm.IsFluxFile() {
						displayFilename = filepath.Join(helm.FluxFile, displayFilename)
					} else {
						displayFilename = filepath.Join(helm.Chart, displayFilename)
					}
				}
				helmName = helm.Name
			}
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		decoder := yaml.NewDecoder(file)
		for {
			// yaml.Node has access to the comments
			// This allows us to get at the Filename comments that Helm leaves
			yamlNodeOriginal := yaml.Node{}

			err = decoder.Decode(&yamlNodeOriginal)
			if err != nil {
				if err != io.EOF {
					return err
				}
				break

			}
			yamlNode := map[string]interface{}{}
			err = yamlNodeOriginal.Decode(&yamlNode)
			if err != nil {
				return err
			}
			kind, ok := yamlNode["kind"].(string)
			if !ok {
				continue
			}
			if kind == "list" {
				nodes := yamlNode["items"].([]interface{})
				for _, node := range nodes {
					obj, ok := node.(map[string]interface{})
					if !ok {
						logrus.Warningf("Found a malformed YAML list item at %s", path+info.Name())
					}
					_, kind, name, namespace := util.ExtractMetadata(obj)
					if kind == "" {
						logrus.Warningf("Found a YAML list item without kind at %s", path+info.Name())
						continue
					}
					if name == "" {
						logrus.Warningf("Found a YAML list item without metadata.name at %s", path+info.Name())
						continue
					}
					newImages, containers := processYamlNode(node.(map[string]interface{}))
					images = append(images, newImages...)
					resources = append(resources, models.Resource{
						Kind:      kind,
						Name:      name,
						Namespace: namespace,
						Filename:  displayFilename,
						HelmName:  helmName,
						Containers: funk.Map(containers, func(c models.Container) string {
							return c.Name
						}).([]string),
					})
				}
			} else {
				_, kind, name, namespace := util.ExtractMetadata(yamlNode)
				if kind == "" {
					logrus.Warningf("Found a YAML file without kind at %s", path+info.Name())
					continue
				}
				if name == "" {
					logrus.Warningf("Found a YAML file without metadata.name at %s", path+info.Name())
					continue
				}
				newImages, containers := processYamlNode(yamlNode)
				images = append(images, newImages...)
				resources = append(resources, models.Resource{
					Kind:      kind,
					Name:      name,
					Namespace: namespace,
					Filename:  displayFilename,
					HelmName:  helmName,
					Containers: funk.Map(containers, func(c models.Container) string {
						return c.Name
					}).([]string),
				})

			}

		}

		return nil
	})
	return images, resources, err
}

func processYamlNode(yamlNode map[string]interface{}) ([]trivymodels.Image, []models.Container) {
	_, kind, name, namespace := util.ExtractMetadata(yamlNode)
	if kind == "" || name == "" {
		return nil, nil
	}
	owner := trivymodels.Resource{
		Kind:      kind,
		Name:      name,
		Namespace: namespace,
	}
	podSpec := GetPodSpec(yamlNode)
	images := getImages(podSpec.(map[string]interface{}))
	return funk.Map(images, func(c models.Container) trivymodels.Image {
		return trivymodels.Image{
			Name: c.Image,
			Owner: trivymodels.Resource{
				Kind:      owner.Kind,
				Container: c.Name,
				Name:      owner.Name,
			},
		}
	}).([]trivymodels.Image), images
}

var podSpecFields = []string{"jobTemplate", "spec", "template"}
var containerSpecFields = []string{"containers", "initContainers"}

// GetPodSpec looks inside arbitrary YAML for a PodSpec
func GetPodSpec(yaml map[string]interface{}) interface{} {
	for _, child := range podSpecFields {
		if childYaml, ok := yaml[child]; ok {
			return GetPodSpec(childYaml.(map[string]interface{}))
		}
	}
	return yaml
}

func getImages(podSpec map[string]interface{}) []models.Container {
	images := make([]models.Container, 0)
	for _, field := range containerSpecFields {
		containerField, ok := podSpec[field]
		if !ok {
			continue
		}
		containers := containerField.([]interface{})
		for _, container := range containers {
			containerMap := container.(map[string]interface{})
			image, _ := containerMap["image"].(string)
			name, _ := containerMap["name"].(string)
			newContainer := models.Container{
				Image: image,
				Name:  name,
			}
			images = append(images, newContainer)
		}
	}
	return images
}

// GetShaAndRepoTags returns the SHA and repo-tags from a tarball of a an image.
func GetShaAndRepoTags(path string) (string, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()

	tarReader := tar.NewReader(f)
	for {
		header, err := tarReader.Next()

		if err != nil {
			if err == io.EOF {
				break
			}
			return "", nil, err
		}
		if header.Name != "manifest.json" {
			continue
		}
		bytes, err := ioutil.ReadAll(tarReader)
		if err != nil {
			return "", nil, err
		}
		jsonBody := make([]interface{}, 0)
		err = json.Unmarshal(bytes, &jsonBody)
		if err != nil {
			return "", nil, err
		}
		allRepoTags := make([]string, 0)
		var configFileName string
		for _, imageDef := range jsonBody {
			configFileName = imageDef.(map[string]interface{})["Config"].(string)
			repoTags := imageDef.(map[string]interface{})["RepoTags"].([]interface{})
			for _, tag := range repoTags {
				allRepoTags = append(allRepoTags, tag.(string))
			}
		}
		sha, err := GetImageSha(path, configFileName)
		if err != nil {
			return "", nil, err
		}
		return sha, allRepoTags, nil
	}
	return "", nil, err
}

// GetImageSha returns the sha from a tarball of a an image.
func GetImageSha(path string, configFileName string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	tarReader := tar.NewReader(f)
	for {
		header, err := tarReader.Next()

		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}
		if header.Name != configFileName {
			continue
		}
		bytes, err := ioutil.ReadAll(tarReader)
		if err != nil {
			return "", err
		}
		var jsonBody interface{}
		err = json.Unmarshal(bytes, &jsonBody)
		if err != nil {
			return "", err
		}
		imageSha := jsonBody.(map[string]interface{})["config"].(map[string]interface{})["Image"]

		if imageSha != nil {
			sha, ok := imageSha.(string)
			if !ok {
				return "", nil
			}
			return sha, nil
		}
	}
	return "", nil
}

// SendResults sends the results to Insights
func SendResults(reports []models.ReportInfo, resources []models.Resource, configurationObject models.Configuration, token string) (*models.ScanResults, error) {
	var b bytes.Buffer

	formFiles := []formFile{{
		field:    "fairwinds-insights",
		filename: "fairwinds-insights.yaml",
		location: "fairwinds-insights.yaml",
	}}
	for _, report := range reports {
		formFiles = append(formFiles, formFile{
			field:    report.Report,
			filename: report.Filename,
			location: configurationObject.Options.TempFolder + "/" + report.Filename,
		})
	}

	w := multipart.NewWriter(&b)
	for _, file := range formFiles {
		fw, err := w.CreateFormFile(file.field, file.filename)
		if err != nil {
			logrus.Fatalf("Unable to create form for %s: %v", file.field, err)
		}
		r, err := os.Open(file.location)
		if err != nil {
			logrus.Fatalf("Unable to open file for %s: %v", file.field, err)
		}
		defer r.Close()
		_, err = io.Copy(fw, r)

		if err != nil {
			logrus.Fatalf("Unable to write contents for %s: %v", file.field, err)
		}
	}
	w.Close()

	repoDetails, err := getGitInfo(configurationObject.Options.RepositoryName, configurationObject.Options.BaseBranch)
	if err != nil {
		logrus.Fatalf("Unable to get git details: %v", err)
	}

	url := fmt.Sprintf("%s/v0/organizations/%s/ci/scan-results", configurationObject.Options.Hostname, configurationObject.Options.Organization)
	req, err := http.NewRequest("POST", url, &b)
	if err != nil {
		logrus.Warn("Unable to create Request")
		return nil, err
	}

	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Commit-Hash", repoDetails.currentHash)
	req.Header.Set("X-Commit-Message", repoDetails.commitMessage)
	req.Header.Set("X-Branch-Name", repoDetails.branch)
	req.Header.Set("X-Master-Hash", repoDetails.masterHash)
	req.Header.Set("X-Base-Branch", configurationObject.Options.BaseBranch)
	req.Header.Set("X-Origin", repoDetails.origin)
	req.Header.Set("X-Repository-Name", repoDetails.repoName)
	req.Header.Set("X-New-AI-Threshold", strconv.Itoa(configurationObject.Options.NewActionItemThreshold))
	req.Header.Set("X-Severity-Threshold", configurationObject.Options.SeverityThreshold)
	req.Header.Set("X-Script-Version", os.Getenv("SCRIPT_VERSION"))
	req.Header.Set("X-Image-Version", os.Getenv("IMAGE_VERSION"))
	for _, report := range reports {
		req.Header.Set("X-Fairwinds-Report-Version-"+report.Report, report.Version)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logrus.Warn("Unable to Post results to Insights")
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Warn("Unable to read results")
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Invalid status code: %d - %s", resp.StatusCode, string(body))
	}

	var results models.ScanResults
	err = json.Unmarshal(body, &results)
	if err != nil {
		logrus.Warn("Unable to unmarshal results")
		return nil, err
	}
	return &results, nil
}

func getGitInfo(repoName, baseBranch string) (gitInfo, error) {
	info := gitInfo{}

	var err error

	masterHash := os.Getenv("MASTER_HASH")
	if masterHash == "" {
		masterHash, err = commands.Exec("git", "merge-base", "HEAD", baseBranch)
		if err != nil {
			logrus.Error("Unable to get GIT merge-base")
			return info, err
		}
	}

	currentHash := os.Getenv("CURRENT_HASH")
	if currentHash == "" {
		currentHash, err = commands.Exec("git", "rev-parse", "HEAD")
		if err != nil {
			logrus.Error("Unable to get GIT Hash")
			return info, err
		}
	}

	commitMessage := os.Getenv("COMMIT_MESSAGE")
	if commitMessage == "" {
		commitMessage, err = commands.Exec("git", "log", "--pretty=format:%s", "-1")
		if err != nil {
			logrus.Error("Unable to get GIT Commit message")
			return info, err
		}
	}
	if len(commitMessage) > 100 {
		commitMessage = commitMessage[:100] // Limit to 100 chars, double the length of github recommended length
	}
	branch := os.Getenv("BRANCH_NAME")
	if branch == "" {
		branch, err = commands.Exec("git", "rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			logrus.Error("Unable to get GIT Branch Name")
			return info, err
		}
	}
	origin := os.Getenv("ORIGIN_URL")
	if origin == "" {
		origin, err = commands.Exec("git", "remote", "get-url", "origin")
		if err != nil {
			logrus.Error("Unable to get GIT Origin")
			return info, err
		}
	}
	if repoName == "" {
		repoName = origin
		if strings.Contains(repoName, "@") { // git@github.com URLs are allowed
			repoNameSplit := strings.Split(repoName, "@")
			// Take the substring after the last @ to avoid any tokens in an HTTPS URL
			repoName = repoNameSplit[len(repoNameSplit)-1]
		} else if strings.Contains(repoName, "//") {
			repoNameSplit := strings.Split(repoName, "//")
			repoName = repoNameSplit[len(repoNameSplit)-1]
		}
		// Remove "******.com:" prefix and ".git" suffix to get clean $org/$repo structure
		if strings.Contains(repoName, ":") {
			repoNameSplit := strings.Split(repoName, ":")
			repoName = repoNameSplit[len(repoNameSplit)-1]
		}
		repoName = strings.TrimSuffix(repoName, ".git")
	}

	info.masterHash = masterHash
	info.currentHash = currentHash
	info.commitMessage = commitMessage
	info.branch = branch
	info.origin = origin
	info.repoName = repoName
	return info, nil
}

// ProcessHelmTemplates turns helm into yaml to be processed by Polaris or the other tools.
func ProcessHelmTemplates(helmConfigs []models.HelmConfig, tempFolder string, configFolder string) error {
	for _, helm := range helmConfigs {
		if helm.IsLocal() && helm.IsRemote() {
			return fmt.Errorf("Error in helm definition %v - It is not possible to use both 'path' and 'repo' simultaneously", helm.Name)
		}
		if helm.IsLocal() {
			err := handleLocalHelmChart(helm, tempFolder, configFolder)
			if err != nil {
				return err
			}
		} else if helm.IsRemote() {
			if helm.IsFluxFile() {
				err := handleFluxHelmChart(helm, tempFolder, configFolder)
				if err != nil {
					return err
				}
			} else {
				err := handleRemoteHelmChart(helm, tempFolder, configFolder)
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

func handleFluxHelmChart(helm models.HelmConfig, tempFolder string, configFolder string) error {
	if helm.Name == "" || helm.Repo == "" {
		return errors.New("Parameters 'name', 'repo' are required when using fluxFile")
	}

	fluxFile, err := os.Open(helm.FluxFile)
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
	return doHandleRemoteHelmChart(helm.Name, helm.Repo, chartName, helmRelease.Spec.Chart.Spec.Version, helm.ValuesFile, helm.Values, helmRelease.Spec.Values, tempFolder, configFolder)
}

func handleRemoteHelmChart(helm models.HelmConfig, tempFolder string, configFolder string) error {
	if helm.Name == "" || helm.Chart == "" || helm.Repo == "" {
		return errors.New("Parameters 'name', 'repo' and 'chart' are required in helm definition")
	}
	return doHandleRemoteHelmChart(helm.Name, helm.Repo, helm.Chart, helm.Version, helm.ValuesFile, helm.Values, nil, tempFolder, configFolder)
}

func doHandleRemoteHelmChart(helmName, repoURL, chartName, chartVersion, valuesFile string, values, fluxValues map[string]interface{}, tempFolder, configFolder string) error {
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

	helmValuesFilePath, err := resolveHelmValuesPath(valuesFile, values, fluxValues, tempFolder)
	if err != nil {
		return err
	}
	return doHandleLocalHelmChart(helmName, chartDownloadPath, helmValuesFilePath, tempFolder, configFolder)
}

func handleLocalHelmChart(helm models.HelmConfig, tempFolder string, configFolder string) error {
	if helm.Name == "" || helm.Path == "" {
		return errors.New("Parameters 'name' and 'path' are required in helm definition")
	}

	helmValuesFilePath, err := resolveHelmValuesPath(helm.ValuesFile, helm.Values, nil, tempFolder)
	if err != nil {
		return err
	}
	return doHandleLocalHelmChart(helm.Name, helm.Path, helmValuesFilePath, tempFolder, configFolder)
}

func doHandleLocalHelmChart(helmName, helmPath, helmValuesFilePath, tempFolder, configFolder string) error {
	_, err := commands.ExecWithMessage(exec.Command("helm", "dependency", "update", helmPath), "Updating dependencies for "+helmName)
	if err != nil {
		return err
	}

	params := []string{"template", helmName, helmPath, "--output-dir", configFolder + helmName, "-f", helmValuesFilePath}
	_, err = commands.ExecWithMessage(exec.Command("helm", params...), "Templating: "+helmName)
	if err != nil {
		return err
	}
	return nil
}

// resolveHelmValuesPath file takes precedence over values
func resolveHelmValuesPath(valuesFile string, values map[string]interface{}, fluxValues map[string]interface{}, tempFolder string) (string, error) {
	hasValuesFile := valuesFile != ""
	hasValues := len(values) > 0
	hasFluxValues := len(fluxValues) > 0

	if hasValuesFile || hasValues || hasFluxValues { // has any
		if !exactlyOneOf(hasValuesFile, hasValues, hasFluxValues) { // if has any, must have exactly one
			return "", fmt.Errorf("only one of valuesFile, values or <fluxFile>.values can be specified")
		}
	}

	if hasValuesFile {
		return valuesFile, nil
	}

	if hasValues {
		yaml, err := yaml.Marshal(values)
		if err != nil {
			return "", err
		}
		valuesFilePath := tempFolder + "helm-values.yaml"
		err = ioutil.WriteFile(valuesFilePath, yaml, 0644)
		if err != nil {
			return "", err
		}
		return valuesFilePath, nil
	}

	if hasFluxValues {
		yaml, err := yaml.Marshal(fluxValues)
		if err != nil {
			return "", err
		}
		valuesFilePath := tempFolder + "flux-helm-values.yaml"
		err = ioutil.WriteFile(valuesFilePath, yaml, 0644)
		if err != nil {
			return "", err
		}
		return valuesFilePath, nil
	}
	return "", nil
}

func exactlyOneOf(inputs ...bool) bool {
	foundAtLeastOne := false
	for _, input := range inputs {
		if input {
			if foundAtLeastOne {
				return false
			}
			foundAtLeastOne = true
		}
	}
	return foundAtLeastOne
}

// CopyYaml adds all Yaml found in a given spot into the manifest folder.
func CopyYaml(configurationObject models.Configuration, configFolder string) error {
	for _, path := range configurationObject.Manifests.YamlPaths {
		_, err := commands.ExecWithMessage(exec.Command("cp", "-r", path, configFolder), "Copying yaml file to config folder")
		if err != nil {
			return err
		}
	}
	return nil
}
