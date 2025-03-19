package ci

import (
	"bytes"
	"context"
	"crypto/tls"
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
	"github.com/hashicorp/go-multierror"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/commands"
	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/util"
)

const configFileName = "fairwinds-insights.yaml"
const maxLinesForPrint = 8

const filesModifiedFileName = "files_modified"

var podSpecFields = []string{"jobTemplate", "spec", "template"}
var containerSpecFields = []string{"containers", "initContainers"}

var ErrExitCode = errors.New("ExitCode is set")

type formFile struct {
	field    string
	filename string
	location string
}

type CIScan struct {
	autoScan       bool
	token          string
	baseFolder     string // . or /app/repository
	repoBaseFolder string // . or /app/repository/{repoName}
	configFolder   string
	config         *models.Configuration
}

type insightsReportConfig struct {
	EnabledOnAutoDiscovery *bool
}

type insightsReportsConfig struct {
	AutoScan map[string]insightsReportConfig
}

// Create a new CI instance based on flag cloneRepo
func NewCIScan(cloneRepo bool, token string) (*CIScan, error) {
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
		autoScan:       cloneRepo,
		token:          token,
		repoBaseFolder: repoBaseFolder,
		baseFolder:     baseFolder,
		configFolder:   configFolder,
		config:         config,
	}

	logrus.Infof("Reports config is opa: %v, polaris: %v, pluto: %v, trivy: %v, tfsec: %v", ci.OPAEnabled(), ci.PolarisEnabled(), ci.PlutoEnabled(), ci.TrivyEnabled(), ci.TerraformEnabled())

	return &ci, nil
}

// Close deletes all temporary folders created.
func (ci *CIScan) Close() {
	os.RemoveAll(ci.config.Options.TempFolder)
	os.RemoveAll(ci.config.Images.FolderName)
}

// getAllResources scans a folder of yaml files and returns all of the images and resources used.
func (ci *CIScan) getAllResources() ([]trivymodels.Image, []models.Resource, error) {
	var images []trivymodels.Image
	var resources []models.Resource
	var errors *multierror.Error
	logrus.Infof("Scanning %s for resources", ci.configFolder)
	err := filepath.Walk(ci.configFolder, func(path string, info os.FileInfo, err error) error {
		if !strings.HasSuffix(info.Name(), ".yaml") && !strings.HasSuffix(info.Name(), ".yml") {
			logrus.Infof("Skipping file %s", path+info.Name())
			return nil
		}

		displayFilename, helmName, err := ci.getDisplayFilenameAndHelmName(path)
		if err != nil {
			errors = multierror.Append(errors, fmt.Errorf("error getting displayFilename and helmName for file %s: %v", path, err))
			logrus.Infof("Skipping file %s", path+info.Name())
			return nil
		}

		fileHandler, err := os.Open(path)
		if err != nil {
			logrus.Infof("Skipping file %s", path+info.Name())
			errors = multierror.Append(errors, fmt.Errorf("error opening file %s: %v", path, err))
			return nil
		}

		decoder := yaml.NewDecoder(fileHandler)
		for {
			// yaml.Node has access to the comments
			// This allows us to get at the Filename comments that Helm leaves
			yamlNodeOriginal := yaml.Node{}

			err = decoder.Decode(&yamlNodeOriginal)
			if err != nil {
				if err != io.EOF {
					logrus.Infof("Skipping file %s", path+info.Name())
					errors = multierror.Append(errors, fmt.Errorf("error decoding file %s: %v", path, err))
					return nil
				}
				break // EOF
			}
			yamlNode := map[string]interface{}{}
			err = yamlNodeOriginal.Decode(&yamlNode)
			if err != nil {
				errors = multierror.Append(errors, fmt.Errorf("error decoding document %s: %v", path, err))
				return nil
			}
			kind, ok := yamlNode["kind"].(string)
			if !ok {
				continue
			}
			logrus.Infof("Processing %s", path+info.Name())
			logrus.Infof("Kind: %s", kind)
			if kind == "list" {
				logrus.Info("Processing YAML NODE LIST", yamlNode)
				nodes := yamlNode["items"].([]interface{})
				for _, node := range nodes {
					obj, ok := node.(map[string]interface{})
					if !ok {
						logrus.Infof("Found a malformed YAML list item at %s", path+info.Name())
						logrus.Warningf("Found a malformed YAML list item at %s", path+info.Name())
					}
					_, kind, name, namespace, labels := util.ExtractMetadata(obj)
					if kind == "" {
						logrus.Infof("Found a YAML list item without kind at %s", path+info.Name())
						logrus.Warningf("Found a YAML list item without kind at %s", path+info.Name())
						continue
					}
					if name == "" {
						logrus.Infof("Found a YAML list item without metadata.name at %s", path+info.Name())
						logrus.Warningf("Found a YAML list item without metadata.name at %s", path+info.Name())
						continue
					}
					newImages, containers := processYamlNode(node.(map[string]interface{}))
					images = append(images, newImages...)
					logrus.Infof("Found %d images in %s", len(newImages), path+info.Name())
					resources = append(resources, models.Resource{
						Kind:      kind,
						Name:      name,
						Namespace: namespace,
						Labels:    labels,
						Filename:  displayFilename,
						HelmName:  helmName,
						Containers: lo.Map(containers, func(c models.Container, _ int) string {
							return c.Name
						}),
					})
				}
			} else {
				logrus.Info("ELSE Processing YAML NODE", yamlNode)
				_, kind, name, namespace, labels := util.ExtractMetadata(yamlNode)
				if kind == "" {
					logrus.Infof("Found a YAML file without kind at %s", path+info.Name())
					logrus.Warningf("Found a YAML file without kind at %s", path+info.Name())
					continue
				}
				if name == "" {
					logrus.Infof("Found a YAML file without metadata.name at %s", path+info.Name())
					logrus.Warningf("Found a YAML file without metadata.name at %s", path+info.Name())
					continue
				}
				newImages, containers := processYamlNode(yamlNode)
				logrus.Infof("Found %d images in %s", len(newImages), path+info.Name())
				images = append(images, newImages...)
				resources = append(resources, models.Resource{
					Kind:      kind,
					Name:      name,
					Namespace: namespace,
					Labels:    labels,
					Filename:  displayFilename,
					HelmName:  helmName,
					Containers: lo.Map(containers, func(c models.Container, _ int) string {
						return c.Name
					}),
				})
			}
		}
		return nil
	})
	if err != nil {
		errors = multierror.Append(errors, fmt.Errorf("error walking directory %s: %v", ci.configFolder, err))
		return nil, nil, errors
	}
	logrus.Infof("AFETR Found %d images in %s", len(images), ci.configFolder)
	// multiple images with the same name may belong to different owners, so we need to deduplicate them
	dedupedImages := dedupImages(images)

	return dedupedImages, resources, errors.ErrorOrNil()
}

func dedupImages(images []trivymodels.Image) []trivymodels.Image {
	imageOwnersMap := map[string][]trivymodels.Resource{}
	for _, img := range images {
		if _, ok := imageOwnersMap[img.Name]; !ok {
			imageOwnersMap[img.Name] = []trivymodels.Resource{} // initialize
		}
		imageOwnersMap[img.Name] = append(imageOwnersMap[img.Name], img.Owners...)
	}

	imagesMap := lo.KeyBy(images, func(img trivymodels.Image) string { return img.Name })

	dedupedImages := []trivymodels.Image{}
	for k, i := range imagesMap {
		dedupedImages = append(dedupedImages, trivymodels.Image{
			ID:                 i.ID,
			PullRef:            i.PullRef,
			Name:               i.Name,
			Owners:             imageOwnersMap[k],
			RecommendationOnly: i.RecommendationOnly,
		})
	}
	logrus.Infof("Deduped %d images to %d", len(images), len(dedupedImages))
	return dedupedImages
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
	_, kind, name, namespace, _ := util.ExtractMetadata(yamlNode)
	if kind == "" || name == "" {
		return nil, nil
	}
	podSpec := getPodSpec(yamlNode)
	images := getImages(podSpec.(map[string]interface{}))
	return lo.Map(images, func(c models.Container, _ int) trivymodels.Image {
		return trivymodels.Image{
			Name: c.Image,
			Owners: []trivymodels.Resource{
				{
					Name:      name,
					Kind:      kind,
					Namespace: namespace,
					Container: c.Name,
				},
			},
		}
	}), images
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
		containers, ok := containerField.([]interface{})
		if !ok {
			logrus.Warningf("Found a podSpec with no containers: %v", podSpec)
			continue
		}
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

	repoDetails, err := getGitInfo(commands.ExecInDir, ci.config.Options.CIRunner, ci.repoBaseFolder, ci.config.Options.RepositoryName, ci.config.Options.BaseBranch)
	if err != nil {
		logrus.Fatalf("Unable to get git details: %v", err)
	}
	if len(repoDetails.filesModified) > 0 {
		fw, err := w.CreateFormFile(filesModifiedFileName, filesModifiedFileName)
		if err != nil {
			logrus.Fatalf("Unable to create form for %s: %v", "files_modified", err)
		}
		_, err = fw.Write([]byte(strings.Join(repoDetails.filesModified, "\n")))
		if err != nil {
			logrus.Fatalf("Unable to write file for %s: %v", "files_modified", err)
		}
	}

	w.Close()

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
	if os.Getenv("SKIP_SSL_VALIDATION") == "true" {
		transport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
		client = &http.Client{Transport: transport}
	}
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
		return getConfigurationForCloneRepo()
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
	logrus.Infof("Running with configuration: %s", config)
	err = config.CheckForErrors()
	if err != nil {
		return "", "", nil, fmt.Errorf("Error parsing fairwinds-insights.yaml: %v", err)
	}
	return filepath.Base(""), filepath.Base(""), config, nil
}

func getConfigurationForCloneRepo() (string, string, *models.Configuration, error) {
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

	var basePath string = os.Getenv("CI_BASE_PATH")
	if basePath != "" {
		logrus.Infof("using basePath of %q from environment CI_BASE_PATH", basePath)
	} else {
		basePath = filepath.Join("/app", "repository")
	}
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
		if strings.TrimSpace(os.Getenv("REPORTS_CONFIG")) != "" {
			err := unmarshalAndOverrideConfig(config)
			if err != nil {
				return "", "", nil, err
			}
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

	config.Options.SetExitCode = false // always false when running on auto-scan mode

	_, err = commands.ExecInDir(baseRepoPath, exec.Command("git", "update-ref", "refs/heads/"+config.Options.BaseBranch, "refs/remotes/origin/"+config.Options.BaseBranch), "updating branch ref")
	if err != nil {
		return "", "", nil, fmt.Errorf("unable to update ref for branch %s: %v", config.Options.BaseBranch, err)
	}

	return filepath.Join(baseRepoPath, "../"), baseRepoPath, config, nil
}

func unmarshalAndOverrideConfig(config *models.Configuration) error {
	var insightsReportConfig insightsReportsConfig
	err := json.Unmarshal([]byte(os.Getenv("REPORTS_CONFIG")), &insightsReportConfig)
	if err != nil {
		return fmt.Errorf("unable to parse auto-scan reports config: %v", err)
	}
	overrideReportsEnabled(config, insightsReportConfig)
	return nil
}

func overrideReportsEnabled(cfg *models.Configuration, reportConfig insightsReportsConfig) {
	if rCfg, ok := reportConfig.AutoScan["opa"]; ok {
		cfg.Reports.OPA.Enabled = rCfg.EnabledOnAutoDiscovery
	}
	if rCfg, ok := reportConfig.AutoScan["polaris"]; ok {
		cfg.Reports.Polaris.Enabled = rCfg.EnabledOnAutoDiscovery
	}
	if rCfg, ok := reportConfig.AutoScan["pluto"]; ok {
		cfg.Reports.Pluto.Enabled = rCfg.EnabledOnAutoDiscovery
	}
	if rCfg, ok := reportConfig.AutoScan["trivy"]; ok {
		cfg.Reports.Trivy.Enabled = rCfg.EnabledOnAutoDiscovery
	}
	if rCfg, ok := reportConfig.AutoScan["tfsec"]; ok {
		cfg.Reports.TFSec.Enabled = rCfg.EnabledOnAutoDiscovery
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
	var scanErrorsReportProperties models.ScanErrorsReportProperties // errors encountered during scan

	err := ci.ProcessHelmTemplates()
	if err != nil {
		scanErrorsReportProperties.AddScanErrorsReportResultFromError(err)
	}

	err = ci.CopyYaml()
	if err != nil {
		scanErrorsReportProperties.AddScanErrorsReportResultFromError(models.ScanErrorsReportResult{
			ErrorMessage: err.Error(),
			ErrorContext: "copying yaml files to configuration directory",
			Kind:         "InternalOperation",
			ResourceName: "CopyYaml",
			Remediation:  "Examine the CI logs to determine why yaml files failed to copy to the configuration directory. Perhaps memory or disk space is low.",
		})
	}

	// Scan YAML, find all images/kind/etc
	manifestImages, resources, err := ci.getAllResources()
	if err != nil {
		scanErrorsReportProperties.AddScanErrorsReportResultFromError(models.ScanErrorsReportResult{
			ErrorMessage: err.Error(),
			ErrorContext: "getting all resources from manifest files",
			Kind:         "InternalOperation",
			ResourceName: "GetAllResources",
		})
	}

	var reports []*models.ReportInfo

	// Scan manifests with Polaris
	if ci.PolarisEnabled() {
		polarisReport, err := ci.GetPolarisReport()
		if err != nil {
			scanErrorsReportProperties.AddScanErrorsReportResultFromError(models.ScanErrorsReportResult{
				ErrorMessage: err.Error(),
				ErrorContext: "running polaris",
				Kind:         "InternalOperation",
				ResourceName: "GetPolarisReport",
			})
		}
		if polarisReport != nil {
			reports = append(reports, polarisReport)
		}
	}

	workloadReport, err := ci.GetWorkloadReport(resources)
	if err != nil {
		return nil, fmt.Errorf("unable to get workloads report, which is depended on by other reports: %v", err)
	}
	if workloadReport != nil {
		reports = append(reports, workloadReport)
	}
	if ci.TrivyEnabled() {
		manifestImagesToScan := manifestImages
		if ci.SkipTrivyManifests() {
			manifestImagesToScan = []trivymodels.Image{}
		}
		dockerImages := getDockerImages(ci.config.Images.Docker, ci.autoScan)
		trivyReport, err := ci.GetTrivyReport(dockerImages, manifestImagesToScan)
		if err != nil {
			scanErrorsReportProperties.AddScanErrorsReportResultFromError(err, models.ScanErrorsReportResult{
				ErrorContext: "downloading images and running trivy",
				Kind:         "InternalOperation",
				ResourceName: "GetTrivyReport",
			})
		}
		if trivyReport != nil {
			reports = append(reports, trivyReport)
		}
	}

	if ci.OPAEnabled() {
		opaReport, err := ci.ProcessOPA(context.Background())
		if err != nil {
			scanErrorsReportProperties.AddScanErrorsReportResultFromError(err, models.ScanErrorsReportResult{
				ErrorContext: "processing OPA policies",
				Kind:         "InternalOperation",
				ResourceName: "ProcessOPA",
			})
		}
		if opaReport != nil {
			reports = append(reports, opaReport)
		}
	}

	if ci.PlutoEnabled() {
		plutoReport, err := ci.GetPlutoReport()
		if err != nil {
			scanErrorsReportProperties.AddScanErrorsReportResultFromError(err, models.ScanErrorsReportResult{
				ErrorContext: "running pluto",
				Kind:         "InternalOperation",
				ResourceName: "GetPlutoReport",
			})
		}
		if plutoReport != nil {
			reports = append(reports, plutoReport)
		}
	}

	if ci.TerraformEnabled() {
		terraformReports, err := ci.ProcessTerraformPaths()
		if err != nil {
			scanErrorsReportProperties.AddScanErrorsReportResultFromError(err, models.ScanErrorsReportResult{
				ErrorContext: "processing terraform",
				Kind:         "InternalOperation",
				ResourceName: "ProcessTerraformPaths",
			})
		}
		if terraformReports != nil {
			logrus.Debugln("the Terraform report contains results")
			reports = append(reports, terraformReports)
		}
	}

	if len(scanErrorsReportProperties.Items) > 0 {
		printScanErrors(scanErrorsReportProperties)
		scanErrorsReport, err := ci.processScanErrorsReportProperties(scanErrorsReportProperties)
		if err != nil {
			return nil, fmt.Errorf("unable to process scan errors report items: %w", err)
		}
		reports = append(reports, scanErrorsReport)
	}
	return reports, nil
}

func hasEnvVar(s string) bool {
	return strings.Contains(s, "$")
}

func getDockerImages(dockerImagesStr []string, autoScan bool) []trivymodels.DockerImage {
	dockerImages := []trivymodels.DockerImage{}
	for _, n := range dockerImagesStr {
		if hasEnvVar(n) {
			if autoScan {
				// only log if on auto-scan, because when running on CI runners - the CI script already have downloaded these images
				logrus.Warnf("image %s skipped, env. variable substitution is not supported on in-container image download", n)
			}
			continue
		}
		dockerImages = append(dockerImages, trivymodels.DockerImage{Name: n})
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

func printScanErrors(scanErrorReport models.ScanErrorsReportProperties) {
	fmt.Println("Scan Errors(these are soft errors that do not prevent the scan from completing):")
	for _, r := range scanErrorReport.Items {
		fmt.Println("\t- " + createErrorItemMessage(r))
	}
}
