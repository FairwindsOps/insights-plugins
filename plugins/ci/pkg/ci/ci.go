package ci

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	trivymodels "github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/models"
	"github.com/sirupsen/logrus"
	"github.com/thoas/go-funk"
	"gopkg.in/yaml.v3"

	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/commands"
	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/util"
)

const configFileName = "fairwinds-insights.yaml"
const maxLinesForPrint = 8

var podSpecFields = []string{"jobTemplate", "spec", "template"}
var containerSpecFields = []string{"containers", "initContainers"}

var ErrExitCode = errors.New("ExitCode is set")

type formFile struct {
	field    string
	filename string
	location string
}

type CIScan struct {
	token          string
	baseFolder     string // . or /app/repository
	repoBaseFolder string // . or /app/repository/{repoName}
	configFolder   string
	config         *models.Configuration
}

type insightsReportConfig struct {
	Enabled *bool
}

type insightsReportsConfig map[string]insightsReportConfig

// Create a new CI instance based on flag cloneRepo
func NewCIScan() (*CIScan, error) {
	cloneRepo := strings.ToLower(strings.TrimSpace(os.Getenv("CLONE_REPO"))) == "true"
	logrus.Infof("cloneRepo: %v", cloneRepo)

	token := strings.TrimSpace(os.Getenv("FAIRWINDS_TOKEN"))
	if token == "" {
		return nil, errors.New("FAIRWINDS_TOKEN environment variable not set")
	}

	baseFolder, repoBaseFolder, config, err := setupConfiguration(cloneRepo)
	if err != nil {
		return nil, fmt.Errorf("could not get configuration: %v", err)
	}

	configFolder := config.Options.TempFolder + "/configuration/"
	err = os.MkdirAll(configFolder, 0755)
	if err != nil {
		return nil, fmt.Errorf("Could not make directory %s: %v", configFolder, err)
	}

	ci := CIScan{
		token:          token,
		repoBaseFolder: repoBaseFolder,
		baseFolder:     baseFolder,
		configFolder:   configFolder,
		config:         config,
	}

	return &ci, nil
}

// Close deletes all temporary folders created.
func (ci *CIScan) Close() {
	os.RemoveAll(ci.config.Options.TempFolder)
	os.RemoveAll(ci.config.Images.FolderName)
}

// getAllResources scans a folder of yaml files and returns all of the images and resources used.
func (ci *CIScan) getAllResources() ([]trivymodels.Image, []models.Resource, error) {
	images := make([]trivymodels.Image, 0)
	resources := make([]models.Resource, 0)
	err := filepath.Walk(ci.configFolder, func(path string, info os.FileInfo, err error) error {
		if !strings.HasSuffix(info.Name(), ".yaml") && !strings.HasSuffix(info.Name(), ".yml") {
			return nil
		}

		displayFilename, helmName, err := ci.getDisplayFilenameAndHelmName(path)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("error opening file %s: %v", path, err)
		}
		decoder := yaml.NewDecoder(file)
		for {
			// yaml.Node has access to the comments
			// This allows us to get at the Filename comments that Helm leaves
			yamlNodeOriginal := yaml.Node{}

			err = decoder.Decode(&yamlNodeOriginal)
			if err != nil {
				if err != io.EOF {
					return fmt.Errorf("error decoding file %s: %v", file.Name(), err)
				}
				break
			}
			yamlNode := map[string]interface{}{}
			err = yamlNodeOriginal.Decode(&yamlNode)
			if err != nil {
				return fmt.Errorf("error decoding[2] file %s: %v", file.Name(), err)
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
	if err != nil {
		return nil, nil, err
	}
	return images, resources, nil
}

func (ci *CIScan) getDisplayFilenameAndHelmName(path string) (string, string, error) {
	var displayFilename, helmName string
	displayFilename, err := filepath.Rel(ci.configFolder, path)
	if err != nil {
		return "", "", fmt.Errorf("cannot be made relative to basepath: %v", err)
	}
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
	return displayFilename, helmName, err
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

// sendResults sends the results to Insights
func (ci *CIScan) sendResults(reports []*models.ReportInfo) (*models.ScanResults, error) {
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

	repoDetails, err := getGitInfo(commands.ExecInDir, ci.config.Options.CIRunner, ci.repoBaseFolder, ci.config.Options.RepositoryName, ci.config.Options.BaseBranch)
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
	req.Header.Set("X-CI-Runner", string(ci.config.Options.CIRunner))
	for _, report := range reports {
		req.Header.Set("X-Fairwinds-Report-Version-"+report.Report, strings.TrimSuffix(report.Version, "\n"))
	}

	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		logrus.Warn("Unable to Post results to Insights")
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
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

// all modifications to config struct must be done in this context
func setupConfiguration(cloneRepo bool) (string, string, *models.Configuration, error) {
	if cloneRepo {
		return getConfigurationForClonedRepo()
	}
	return getDefaultConfiguration()
}

func getDefaultConfiguration() (string, string, *models.Configuration, error) {
	// i.e.: ./fairwinds-insights.yaml
	config, err := readConfigurationFromFile("./" + configFileName)
	if err != nil {
		if !os.IsNotExist(errors.Unwrap(err)) {
			return "", "", nil, err
		}
		logrus.Infof("Could not detect fairwinds-insights.yaml file... auto-detecting...")
		config, err = ConfigFileAutoDetection("")
		if err != nil {
			return "", "", nil, err
		}

		err := createFileFromConfig("", configFileName, *config)
		if err != nil {
			return "", "", nil, err
		}
	}
	err = config.SetDefaults()
	if err != nil {
		return "", "", nil, err
	}
	config.SetPathDefaults()
	logrus.Infof("Running with configuration %#v", config)
	err = config.CheckForErrors()
	if err != nil {
		return "", "", nil, fmt.Errorf("Error parsing fairwinds-insights.yaml: %v", err)
	}
	return filepath.Base(""), filepath.Base(""), config, nil
}

func getConfigurationForClonedRepo() (string, string, *models.Configuration, error) {
	repoFullName := strings.TrimSpace(os.Getenv("REPOSITORY_NAME"))
	if repoFullName == "" {
		return "", "", nil, errors.New("REPOSITORY_NAME environment variable not set")
	}

	branch := strings.TrimSpace(os.Getenv("BRANCH_NAME"))
	if branch == "" {
		return "", "", nil, errors.New("BRANCH environment variable not set")
	}

	if strings.TrimSpace(os.Getenv("IMAGE_VERSION")) == "" {
		return "", "", nil, errors.New("IMAGE_VERSION environment variable not set")
	}

	basePath := filepath.Join("/app", "repository")
	_, repoName := util.GetRepoDetails(repoFullName)
	baseRepoPath := filepath.Join(basePath, repoName)

	err := os.RemoveAll(baseRepoPath)
	if err != nil {
		return "", "", nil, fmt.Errorf("unable to delete existing directory: %v", err)
	}

	url := fmt.Sprintf("https://@github.com/%s", repoFullName)
	accessToken := strings.TrimSpace(os.Getenv("GITHUB_ACCESS_TOKEN"))
	if accessToken != "" {
		// access token is required for private repos
		url = fmt.Sprintf("https://x-access-token:%s@github.com/%s", accessToken, repoFullName)
	}

	_, err = commands.ExecInDir(basePath, exec.Command("git", "clone", "--branch", branch, url), "cloning github repository")
	if err != nil {
		return "", "", nil, fmt.Errorf("unable to clone repository: %v", err)
	}

	// i.e.: /app/repository/blog/fairwinds-insights.yaml
	configFilePath := filepath.Join(basePath, repoName, configFileName)

	config, err := readConfigurationFromFile(configFilePath)
	if err != nil {
		if !os.IsNotExist(errors.Unwrap(err)) {
			return "", "", nil, err
		}
		logrus.Infof("Could not detect fairwinds-insights.yaml file... auto-detecting...")
		config, err = ConfigFileAutoDetection(baseRepoPath)
		if err != nil {
			return "", "", nil, err
		}

		// this is how we support enabling/disabling reports on auto-discovery (when no fairwinds-insights.yaml file is found)
		if strings.TrimSpace(os.Getenv("AUTO_SCAN_REPORTS_CONFIG")) != "" {
			var insightsReportConfig insightsReportsConfig
			err := json.Unmarshal([]byte(os.Getenv("AUTO_SCAN_REPORTS_CONFIG")), &insightsReportConfig)
			if err != nil {
				return "", "", nil, fmt.Errorf("unable to parse auto-scan reports config: %v", err)
			}
			overrideReportsEnabled(config, insightsReportConfig)
		}

		err := createFileFromConfig(baseRepoPath, configFileName, *config)
		if err != nil {
			return "", "", nil, err
		}
	}
	err = config.SetDefaults()
	if err != nil {
		return "", "", nil, err
	}

	err = config.SetMountedPathDefaults(basePath, baseRepoPath)
	if err != nil {
		return "", "", nil, fmt.Errorf("Could not set set path defaults correctly: %v", err)
	}

	err = config.CheckForErrors()
	if err != nil {
		return "", "", nil, fmt.Errorf("Error parsing fairwinds-insights.yaml: %v", err)
	}

	_, err = commands.ExecInDir(baseRepoPath, exec.Command("git", "update-ref", "refs/heads/"+config.Options.BaseBranch, "refs/remotes/origin/"+config.Options.BaseBranch), "updating branch ref")
	if err != nil {
		return "", "", nil, fmt.Errorf("unable to update ref for branch %s: %v", config.Options.BaseBranch, err)
	}

	return filepath.Join(baseRepoPath, "../"), baseRepoPath, config, nil
}

func overrideReportsEnabled(cfg *models.Configuration, reportConfig insightsReportsConfig) {
	if rCfg, ok := reportConfig["opa"]; ok {
		cfg.Reports.OPA.Enabled = rCfg.Enabled
	}
	if rCfg, ok := reportConfig["polaris"]; ok {
		cfg.Reports.Polaris.Enabled = rCfg.Enabled
	}
	if rCfg, ok := reportConfig["pluto"]; ok {
		cfg.Reports.Pluto.Enabled = rCfg.Enabled
	}
	if rCfg, ok := reportConfig["trivy"]; ok {
		cfg.Reports.Trivy.Enabled = rCfg.Enabled
	}
	if rCfg, ok := reportConfig["tfsec"]; ok {
		cfg.Reports.TFSec.Enabled = rCfg.Enabled
	}
}

func readConfigurationFromFile(configFilePath string) (*models.Configuration, error) {
	configHandler, err := os.Open(configFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("Please add fairwinds-insights.yaml to the base of your repository: %w", err)
		} else {
			return nil, fmt.Errorf("Could not open fairwinds-insights.yaml: %v", err)
		}
	}
	return readConfigurationFromReader(configHandler)
}

func readConfigurationFromReader(configHandler io.Reader) (*models.Configuration, error) {
	configContents, err := io.ReadAll(configHandler)
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

func (ci *CIScan) ProcessRepository() ([]*models.ReportInfo, error) {
	err := ci.ProcessHelmTemplates()
	if err != nil {
		return nil, fmt.Errorf("Error while processing helm templates: %v", err)
	}

	err = ci.CopyYaml()
	if err != nil {
		return nil, fmt.Errorf("Error while copying YAML files: %v", err)
	}

	// Scan YAML, find all images/kind/etc
	manifestImages, resources, err := ci.getAllResources()
	if err != nil {
		return nil, fmt.Errorf("Error while extracting images from YAML manifests: %v", err)
	}

	var reports []*models.ReportInfo

	// Scan manifests with Polaris
	if ci.PolarisEnabled() {
		polarisReport, err := ci.GetPolarisReport()
		if err != nil {
			return nil, fmt.Errorf("Error while running Polaris: %v", err)
		}
		reports = append(reports, &polarisReport)
	}

	if ci.TrivyEnabled() {
		manifestImagesToScan := manifestImages
		if ci.SkipTrivyManifests() {
			manifestImagesToScan = []trivymodels.Image{}
		}
		dockerImages := getDockerImages(ci.config.Images.Docker)
		trivyReport, err := ci.GetTrivyReport(dockerImages, manifestImagesToScan)
		if err != nil {
			return nil, fmt.Errorf("Error while running Trivy: %v", err)
		}
		reports = append(reports, trivyReport)
	}

	workloadReport, err := ci.GetWorkloadReport(resources)
	if err != nil {
		return nil, fmt.Errorf("Error while aggregating workloads: %v", err)
	}
	reports = append(reports, &workloadReport)

	if ci.OPAEnabled() {
		opaReport, err := ci.ProcessOPA(context.Background())
		if err != nil {
			return nil, fmt.Errorf("Error while running OPA: %v", err)
		}
		reports = append(reports, &opaReport)
	}

	if ci.PlutoEnabled() {
		plutoReport, err := ci.GetPlutoReport()
		if err != nil {
			return nil, fmt.Errorf("Error while running Pluto: %v", err)
		}
		reports = append(reports, &plutoReport)
	}

	if ci.TerraformEnabled() {
		terraformReports, areTerraformResults, err := ci.ProcessTerraformPaths()
		if err != nil {
			return nil, fmt.Errorf("while processing Terraform: %w", err)
		}
		if areTerraformResults {
			logrus.Debugln("the Terraform report contains results")
			reports = append(reports, &terraformReports)
		}
	}
	return reports, nil
}

func getDockerImages(dockerImagesStr []string) []trivymodels.DockerImage {
	dockerImages := []trivymodels.DockerImage{}
	for _, v := range dockerImagesStr {
		dockerImages = append(dockerImages, trivymodels.DockerImage{
			Name: v,
		})
	}
	return dockerImages
}

func (ci *CIScan) SendAndPrintResults(reports []*models.ReportInfo) error {
	ci.printScannedFilesInfo()

	results, err := ci.sendResults(reports)
	if err != nil {
		return fmt.Errorf("Error while sending results back to %s: %v", ci.config.Options.Hostname, err)
	}
	fmt.Printf("%d new Action Items:\n", len(results.NewActionItems))
	printActionItems(results.NewActionItems)
	fmt.Printf("%d fixed Action Items:\n", len(results.FixedActionItems))
	printActionItems(results.FixedActionItems)

	if ci.JUnitEnabled() {
		err = ci.SaveJUnitFile(*results)
		if err != nil {
			return fmt.Errorf("Could not save jUnit results: %v", err)
		}
	}

	if !results.Pass {
		fmt.Printf("\n\nFairwinds Insights checks failed:\n%v\n\nVisit %s/orgs/%s/repositories for more information\n\n", err, ci.config.Options.Hostname, ci.config.Options.Organization)
		if ci.config.Options.SetExitCode {
			return ErrExitCode
		}
	} else {
		fmt.Println("\n\nFairwinds Insights checks passed.")
	}
	return nil
}

func (ci *CIScan) printScannedFilesInfo() {
	s := len(ci.config.Manifests.YamlPaths)
	if s > 0 {
		fmt.Println("Kubernetes files scanned:")
		for i, p := range ci.config.Manifests.YamlPaths {
			fmt.Printf("\t[%d/%d] - %s\n", i+1, s, p)
		}
	}

	s = len(ci.config.Manifests.Helm)
	if s > 0 {
		fmt.Println("Helm charts scanned:")
		for i, h := range ci.config.Manifests.Helm {
			fmt.Printf("\t[%d/%d] - %s/%s\n", i+1, s, h.Path, h.Name)
		}
	}

	s = len(ci.config.Terraform.Paths)
	if s > 0 {
		fmt.Println("Terraform files scanned:")
		for i, p := range ci.config.Terraform.Paths {
			fmt.Printf("\t[%d/%d] - %s\n", i+1, s, p)
		}
	}
}

func printActionItems(ais []models.ActionItem) {
	for _, ai := range ais {
		fmt.Println(ai.GetReadableTitle())
		printMultilineString("Description", ai.Description)
		printMultilineString("Remediation", ai.Remediation)
		fmt.Println()
	}
}

func printMultilineString(title, str string) {
	fmt.Println("  " + title + ":")
	if str == "" {
		str = "Unspecified"
	}
	lines := strings.Split(str, "\n")
	for idx, line := range lines {
		fmt.Println("    " + line)
		if idx == maxLinesForPrint {
			fmt.Println("    [truncated]")
			break
		}
	}
}

func createFileFromConfig(path, filename string, cfg models.Configuration) error {
	bytes, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	err = os.WriteFile(filepath.Join(path, filename), bytes, 0644)
	if err != nil {
		return err
	}
	return nil
}
