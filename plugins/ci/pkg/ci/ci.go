package ci

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"encoding/xml"
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
	"github.com/jstemmer/go-junit-report/formatter"
	"github.com/sirupsen/logrus"
	"github.com/thoas/go-funk"
	"gopkg.in/yaml.v3"

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

// GetResultsFromCommand executes a command and returns the results as a string.
func GetResultsFromCommand(command string, args ...string) (string, error) {
	bytes, err := exec.Command(command, args...).Output()
	if err != nil {
		logrus.Errorf("Unable to execute command: %v %v", command, strings.Join(args, " "))
		return "", err
	}
	return strings.TrimSpace(string(bytes)), err
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
					displayFilename = filepath.Join(helm.Chart, displayFilename)
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

// GetShaAndRepoTags returns the SHA and repotags from a tarball of a an image.
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
		var sha string
		sha, ok := jsonBody.(map[string]interface{})["config"].(map[string]interface{})["Image"].(string)
		if !ok {
			logrus.Warnf("could not find image sha for path: %v, configFileName: %v and body %+v", path, configFileName, jsonBody)
			return "", nil
		}
		return sha, nil
	}
	return "", nil
}

// SendResults sends the results to Insights
func SendResults(reports []models.ReportInfo, resources []models.Resource, configurationObject models.Configuration, token string) (models.ScanResults, error) {
	var b bytes.Buffer
	var results models.ScanResults

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
		return results, err
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
		return results, err
	}
	if resp.StatusCode != http.StatusOK {
		return results, fmt.Errorf("Invalid status code: %d", resp.StatusCode)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Warn("Unable to read results")
		return results, err
	}
	err = json.Unmarshal(body, &results)
	if err != nil {
		return results, err
	}

	return results, nil
}

func getGitInfo(repoName, baseBranch string) (gitInfo, error) {
	info := gitInfo{}

	_, err := GetResultsFromCommand("git", "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return info, fmt.Errorf("%v: %v", "Please be sure to run the insights-ci script inside of a valid git repository, with the branch you are scanning checked out", err)
	}

	masterHash, err := GetResultsFromCommand("git", "merge-base", "HEAD", baseBranch)
	if err != nil {
		logrus.Error("Unable to get GIT merge-base")
		return info, err
	}

	currentHash, err := GetResultsFromCommand("git", "rev-parse", "HEAD")
	if err != nil {
		logrus.Error("Unable to get GIT Hash")
		return info, err
	}

	commitMessage, err := GetResultsFromCommand("git", "log", "--pretty=format:%s", "-1")
	if err != nil {
		logrus.Error("Unable to get GIT Commit message")
		return info, err
	}
	if len(commitMessage) > 100 {
		commitMessage = commitMessage[:100] // Limit to 100 chars, double the length of github recommended length
	}

	branch, err := GetResultsFromCommand("git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		logrus.Error("Unable to get GIT Branch Name")
		return info, err
	}

	origin, err := GetResultsFromCommand("git", "remote", "get-url", "origin")
	if err != nil {
		logrus.Error("Unable to get GIT Origin")
		return info, err
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

// SaveJUnitFile will save the
func SaveJUnitFile(results models.ScanResults, filename string) error {
	cases := make([]formatter.JUnitTestCase, 0)

	for _, actionItem := range results.NewActionItems {
		cases = append(cases, formatter.JUnitTestCase{
			Name: actionItem.GetReadableTitle(),
			Failure: &formatter.JUnitFailure{
				Message:  actionItem.Remediation,
				Contents: fmt.Sprintf("File: %s\nDescription: %s", actionItem.Notes, actionItem.Description),
			},
		})
	}

	for _, actionItem := range results.FixedActionItems {
		cases = append(cases, formatter.JUnitTestCase{
			Name: actionItem.GetReadableTitle(),
		})
	}

	testSuites := formatter.JUnitTestSuites{
		Suites: []formatter.JUnitTestSuite{
			{
				Tests:     len(results.NewActionItems) + len(results.FixedActionItems),
				TestCases: cases,
			},
		},
	}

	err := os.MkdirAll(filepath.Dir(filename), 0644)
	if err != nil {
		return err
	}

	xmlBytes, err := xml.MarshalIndent(testSuites, "", "\t")
	if err != nil {
		return err
	}
	xmlBytes = append([]byte(xml.Header), xmlBytes...)
	err = ioutil.WriteFile(filename, xmlBytes, 0644)
	if err != nil {
		return err
	}

	return nil
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
			err := handleRemoteHelmChart(helm, tempFolder, configFolder)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func handleRemoteHelmChart(helm models.HelmConfig, tempFolder string, configFolder string) error {
	if helm.Repo == "" || helm.Chart == "" || helm.Name == "" {
		return errors.New("Parameters 'name', 'repo' and 'chart' are required in helm definition")
	}

	repoName := helm.Chart + "-repo"

	err := util.RunCommand(exec.Command("helm", "repo", "add", repoName, helm.Repo), "Adding chart repository: "+repoName)
	if err != nil {
		return err
	}

	repoDownloadPath := fmt.Sprintf("%s/downloaded-charts/%s/", tempFolder, repoName)
	chartDownloadPath := repoDownloadPath + helm.Chart
	err = os.RemoveAll(chartDownloadPath)
	if err != nil {
		return err
	}
	chartFullName := fmt.Sprintf("%s/%s", repoName, helm.Chart)
	err = util.RunCommand(exec.Command("helm", "fetch", chartFullName, "--untar", "--destination", repoDownloadPath), fmt.Sprintf("Retrieving pkg %v from repository %v, downloading it locally and unziping it", helm.Chart, repoName))
	if err != nil {
		return err
	}

	// set helm.Path to the chart downloaded path to be able to reuse handleLocalHelmChart
	helm.Path = chartDownloadPath
	return handleLocalHelmChart(helm, tempFolder, configFolder)
}

func handleLocalHelmChart(helm models.HelmConfig, tempFolder string, configFolder string) error {
	err := util.RunCommand(exec.Command("helm", "dependency", "update", helm.Path), "Updating dependencies for "+helm.Name)
	if err != nil {
		return err
	}
	helmValuesFilePath, err := resolveHelmValuesPath(helm, tempFolder)
	if err != nil {
		return err
	}

	params := []string{"template", helm.Name, helm.Path, "--output-dir", configFolder + helm.Name, "-f", helmValuesFilePath}
	err = util.RunCommand(exec.Command("helm", params...), "Templating: "+helm.Name)
	if err != nil {
		return err
	}
	return nil
}

// resolveHelmValuesPath file takes precedence over values
func resolveHelmValuesPath(helm models.HelmConfig, tempFolder string) (string, error) {
	if helm.ValuesFile != "" {
		return helm.ValuesFile, nil
	}

	valuesFilePath := tempFolder + "helmValues.yaml"
	yaml, err := yaml.Marshal(helm.Values)
	if err != nil {
		return "", err
	}
	err = ioutil.WriteFile(valuesFilePath, yaml, 0644)
	if err != nil {
		return "", err
	}
	return valuesFilePath, nil
}

// CopyYaml adds all Yaml found in a given spot into the manifest folder.
func CopyYaml(configurationObject models.Configuration, configFolder string) error {
	for _, path := range configurationObject.Manifests.YamlPaths {
		err := util.RunCommand(exec.Command("cp", "-r", path, configFolder), "Copying yaml file to config folder")
		if err != nil {
			return err
		}
	}
	return nil
}
