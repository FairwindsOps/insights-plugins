package ci

import (
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

const configFileName = "fairwinds-insights.yaml"

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

type CI struct {
	token          string
	baseFolder     string // . or /app/repository
	repoBaseFolder string // . or /app/repository/{repoName}
	configFolder   string
	config         *models.Configuration
}

// Create a new CI instance based on flag cloneRepo
func NewCI(cloneRepo bool) (*CI, func(), error) {
	logrus.Infof("cloneRepo: %v", cloneRepo)
	token := strings.TrimSpace(os.Getenv("FAIRWINDS_TOKEN"))
	if token == "" {
		return nil, func() {}, errors.New("FAIRWINDS_TOKEN environment variable not set")
	}
	repoBaseFolder, config, err := setupConfiguration(cloneRepo)
	if err != nil {
		return nil, func() {}, fmt.Errorf("could not get configuration: %v", err)
	}

	cleanUpFn := func() {
		os.RemoveAll(config.Options.TempFolder)
		os.RemoveAll(config.Images.FolderName)
	}

	configFolder := config.Options.TempFolder + "/configuration/"
	err = os.MkdirAll(configFolder, 0755)
	if err != nil {
		return nil, cleanUpFn, fmt.Errorf("Could not make directory %s: %v", configFolder, err)
	}

	ci := CI{
		token:          token,
		repoBaseFolder: repoBaseFolder,
		baseFolder:     filepath.Join(repoBaseFolder, "../"),
		configFolder:   configFolder,
		config:         config,
	}

	logrus.Infof("%+v", ci)

	return &ci, cleanUpFn, nil
}

// GetAllResources scans a folder of yaml files and returns all of the images and resources used.
func (ci *CI) GetAllResources() ([]trivymodels.Image, []models.Resource, error) {
	images := make([]trivymodels.Image, 0)
	resources := make([]models.Resource, 0)
	err := filepath.Walk(ci.configFolder, func(path string, info os.FileInfo, err error) error {
		if !strings.HasSuffix(info.Name(), ".yaml") && !strings.HasSuffix(info.Name(), ".yml") {
			return nil
		}

		displayFilename, err := filepath.Rel(ci.configFolder, path)
		if err != nil {
			return err
		}
		var helmName string
		for _, helm := range ci.config.Manifests.Helm {
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
	podSpec := getPodSpec(yamlNode)
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

// getPodSpec looks inside arbitrary YAML for a PodSpec
func getPodSpec(yaml map[string]interface{}) interface{} {
	for _, child := range podSpecFields {
		if childYaml, ok := yaml[child]; ok {
			return getPodSpec(childYaml.(map[string]interface{}))
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

// SendResults sends the results to Insights
func (ci *CI) SendResults(reports []models.ReportInfo, resources []models.Resource) (*models.ScanResults, error) {
	var b bytes.Buffer

	formFiles := []formFile{{
		field:    "fairwinds-insights",
		filename: configFileName,
		location: filepath.Join(ci.repoBaseFolder, configFileName),
	}}
	for _, report := range reports {
		formFiles = append(formFiles, formFile{
			field:    report.Report,
			filename: report.Filename,
			location: filepath.Join(ci.config.Options.TempFolder, report.Filename),
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

	repoDetails, err := getGitInfo(ci.repoBaseFolder, ci.config.Options.RepositoryName, ci.config.Options.BaseBranch)
	if err != nil {
		logrus.Fatalf("Unable to get git details: %v", err)
	}

	url := fmt.Sprintf("%s/v0/organizations/%s/ci/scan-results", ci.config.Options.Hostname, ci.config.Options.Organization)
	req, err := http.NewRequest("POST", url, &b)
	if err != nil {
		logrus.Warn("Unable to create Request")
		return nil, err
	}

	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+ci.token)
	req.Header.Set("X-Commit-Hash", repoDetails.currentHash)
	req.Header.Set("X-Commit-Message", repoDetails.commitMessage)
	req.Header.Set("X-Branch-Name", repoDetails.branch)
	req.Header.Set("X-Master-Hash", repoDetails.masterHash)
	req.Header.Set("X-Base-Branch", ci.config.Options.BaseBranch)
	req.Header.Set("X-Origin", repoDetails.origin)
	req.Header.Set("X-Repository-Name", repoDetails.repoName)
	req.Header.Set("X-New-AI-Threshold", strconv.Itoa(ci.config.Options.NewActionItemThreshold))
	req.Header.Set("X-Severity-Threshold", ci.config.Options.SeverityThreshold)
	req.Header.Set("X-Script-Version", os.Getenv("SCRIPT_VERSION"))
	req.Header.Set("X-Image-Version", os.Getenv("IMAGE_VERSION"))
	for _, report := range reports {
		req.Header.Set("X-Fairwinds-Report-Version-"+report.Report, strings.TrimSuffix(report.Version, "\n"))
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

func getGitInfo(baseRepoPath, repoName, baseBranch string) (*gitInfo, error) {
	var err error
	masterHash := os.Getenv("MASTER_HASH")
	if masterHash == "" {
		masterHash, err = commands.ExecInDir(baseRepoPath, exec.Command("git", "merge-base", "HEAD", baseBranch), "getting master hash")
		if err != nil {
			logrus.Error("Unable to get GIT merge-base")
			return nil, err
		}
	}

	currentHash := os.Getenv("CURRENT_HASH")
	if currentHash == "" {
		currentHash, err = commands.ExecInDir(baseRepoPath, exec.Command("git", "rev-parse", "HEAD"), "getting current hash")
		if err != nil {
			logrus.Error("Unable to get GIT Hash")
			return nil, err
		}
	}

	commitMessage := os.Getenv("COMMIT_MESSAGE")
	if commitMessage == "" {
		commitMessage, err = commands.ExecInDir(baseRepoPath, exec.Command("git", "log", "--pretty=format:%s", "-1"), "getting commit message")
		if err != nil {
			logrus.Error("Unable to get GIT Commit message")
			return nil, err
		}
	}
	if len(commitMessage) > 100 {
		commitMessage = commitMessage[:100] // Limit to 100 chars, double the length of github recommended length
	}
	branch := os.Getenv("BRANCH_NAME")
	if branch == "" {
		branch, err = commands.ExecInDir(baseRepoPath, exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD"), "getting branch name")
		if err != nil {
			logrus.Error("Unable to get GIT Branch Name")
			return nil, err
		}
	}
	origin := os.Getenv("ORIGIN_URL")
	if origin == "" {
		origin, err = commands.ExecInDir(baseRepoPath, exec.Command("git", "remote", "get-url", "origin"), "getting origin url")
		if err != nil {
			logrus.Error("Unable to get GIT Origin")
			return nil, err
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
	return &gitInfo{
		masterHash:    strings.TrimSuffix(masterHash, "\n"),
		currentHash:   strings.TrimSuffix(currentHash, "\n"),
		commitMessage: strings.TrimSuffix(commitMessage, "\n"),
		branch:        strings.TrimSuffix(branch, "\n"),
		origin:        strings.TrimSuffix(origin, "\n"),
		repoName:      strings.TrimSuffix(repoName, "\n"),
	}, nil
}

// ProcessHelmTemplates turns helm into yaml to be processed by Polaris or the other tools.
func (ci *CI) ProcessHelmTemplates() error {
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
				err := handleFluxHelmChart(helm, ci.config.Options.TempFolder, ci.configFolder)
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

func handleLocalHelmChart(helm models.HelmConfig, baseRepoFolder, tempFolder string, configFolder string) error {
	if helm.Name == "" || helm.Path == "" {
		return errors.New("Parameters 'name' and 'path' are required in helm definition")
	}

	helmValuesFilePath, err := resolveHelmValuesPath(helm.ValuesFile, helm.Values, nil, tempFolder)
	if err != nil {
		return err
	}
	return doHandleLocalHelmChart(helm.Name, filepath.Join(baseRepoFolder, helm.Path), helmValuesFilePath, tempFolder, configFolder)
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

func resolveHelmValuesPath(valuesFile string, values map[string]interface{}, fluxValues map[string]interface{}, tempFolder string) (string, error) {
	hasValuesFile := valuesFile != ""
	hasValues := len(values) > 0
	hasFluxValues := len(fluxValues) > 0

	if hasValuesFile || hasValues || hasFluxValues { // has any
		if !util.ExactlyOneOf(hasValuesFile, hasValues, hasFluxValues) { // if has any, must have exactly one
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

// CopyYaml adds all Yaml found in a given spot into the manifest folder.
func (ci *CI) CopyYaml() error {
	for _, path := range ci.config.Manifests.YamlPaths {
		_, err := commands.ExecWithMessage(exec.Command("cp", "-r", filepath.Join(ci.repoBaseFolder, path), ci.configFolder), "Copying yaml file to config folder")
		if err != nil {
			return err
		}
	}
	return nil
}

// all modifications to config struct must be done in this context
func setupConfiguration(cloneRepo bool) (string, *models.Configuration, error) {
	if cloneRepo {
		return getConfigurationForClonedRepo()
	}
	return getDefaultConfiguration()
}

func getDefaultConfiguration() (string, *models.Configuration, error) {
	// i.e.: ./fairwinds-insights.yaml
	config, err := readConfigurationFromFile("./" + configFileName)
	if err != nil {
		return "", nil, err
	}
	config.SetDefaults()
	config.SetPathDefaults()

	err = config.CheckForErrors()
	if err != nil {
		return "", nil, fmt.Errorf("Error parsing fairwinds-insights.yaml: %v", err)
	}
	return filepath.Base(""), config, nil
}

func getConfigurationForClonedRepo() (string, *models.Configuration, error) {
	repoFullName := strings.TrimSpace(os.Getenv("REPOSITORY_NAME"))
	if repoFullName == "" {
		return "", nil, errors.New("REPOSITORY_NAME environment variable not set")
	}

	branch := strings.TrimSpace(os.Getenv("BRANCH"))
	if branch == "" {
		return "", nil, errors.New("BRANCH environment variable not set")
	}

	if strings.TrimSpace(os.Getenv("IMAGE_VERSION")) == "" {
		return "", nil, errors.New("IMAGE_VERSION environment variable not set")
	}

	basePath := filepath.Join("/app", "repository")
	_, repoName := util.GetRepoDetails(repoFullName)
	baseRepoPath := filepath.Join(basePath, repoName)

	err := os.RemoveAll(baseRepoPath)
	if err != nil {
		return "", nil, fmt.Errorf("unable to delete existing directory: %v", err)
	}

	url := fmt.Sprintf("https://@github.com/%s", repoFullName)
	accessToken := strings.TrimSpace(os.Getenv("ACCESS_TOKEN"))
	if accessToken != "" {
		// access token is required for private repos
		url = fmt.Sprintf("https://x-access-token:%s@github.com/%s", accessToken, repoFullName)
	}

	_, err = commands.ExecInDir(basePath, exec.Command("git", "clone", "--branch", branch, url), "cloning github repository")
	if err != nil {
		return "", nil, fmt.Errorf("unable to clone repository: %v", err)
	}

	// i.e.: /app/repository/blog/fairwinds-insights.yaml
	configFilePath := filepath.Join(basePath, repoName, configFileName)

	config, err := readConfigurationFromFile(configFilePath)
	if err != nil {
		return "", nil, err
	}
	config.SetDefaults()
	err = config.SetMountedPathDefaults(basePath, baseRepoPath)
	if err != nil {
		return "", nil, fmt.Errorf("Could not set set path defaults correctly: %v", err)
	}

	err = config.CheckForErrors()
	if err != nil {
		return "", nil, fmt.Errorf("Error parsing fairwinds-insights.yaml: %v", err)
	}

	_, err = commands.ExecInDir(baseRepoPath, exec.Command("git", "update-ref", "refs/heads/"+config.Options.BaseBranch, "refs/remotes/origin/"+config.Options.BaseBranch), "updating branch ref")
	if err != nil {
		return "", nil, fmt.Errorf("unable to update ref for branch %s: %v", config.Options.BaseBranch, err)
	}

	return baseRepoPath, config, nil
}

func readConfigurationFromFile(configFilePath string) (*models.Configuration, error) {
	configHandler, err := os.Open(configFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("Please add fairwinds-insights.yaml to the base of your repository.")
		} else {
			return nil, fmt.Errorf("Could not open fairwinds-insights.yaml: %v", err)
		}
	}
	configContents, err := ioutil.ReadAll(configHandler)
	if err != nil {
		return nil, fmt.Errorf("Could not read fairwinds-insights.yaml: %v", err)
	}
	config := models.Configuration{}
	err = yaml.Unmarshal(configContents, &config)
	if err != nil {
		return nil, fmt.Errorf("Could not parse fairwinds-insights.yaml: %v", err)
	}
	return &config, nil
}

// Hostname return the configured hostname
func (ci *CI) Hostname() string {
	return ci.config.Options.Hostname
}

// ExitCode return if the exitCode flag is set
func (ci *CI) ExitCode() bool {
	return ci.config.Options.SetExitCode
}

// Organization returns the configured organization
func (ci *CI) Organization() string {
	return ci.config.Options.Organization
}

// RepositoryName returns the name of the repository
func (ci *CI) RepositoryName() string {
	return ci.config.Options.RepositoryName
}
