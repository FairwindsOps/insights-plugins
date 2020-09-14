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

// GetResultsFromCommand executes a command and returns the results as a string.
func GetResultsFromCommand(command string, args ...string) (string, error) {
	bytes, err := exec.Command(command, args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(bytes)), err
}

// GetImagesFromManifest scans a folder of yaml files and returns all of the images used.
func GetImagesFromManifest(configFolder string) ([]trivymodels.Image, []models.Resource, error) {
	images := make([]trivymodels.Image, 0)
	resources := make([]models.Resource, 0)
	err := filepath.Walk(configFolder, func(path string, info os.FileInfo, err error) error {
		if !strings.HasSuffix(info.Name(), ".yaml") {
			return nil
		}
		relativePath, err := filepath.Rel(configFolder, path)
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
			kind := yamlNode["kind"].(string)
			fileName := relativePath
			if yamlNodeOriginal.HeadComment != "" {
				fileName = fileName + " > " + yamlNodeOriginal.HeadComment
			}
			if kind == "list" {
				nodes := yamlNode["items"].([]interface{})
				for _, node := range nodes {
					metadata := node.(map[string]interface{})["metadata"].(map[string]interface{})
					namespace := ""
					if namespaceObj, ok := metadata["namespace"]; ok {
						namespace = namespaceObj.(string)
					}
					newImages, containers := processYamlNode(node.(map[string]interface{}))
					images = append(images, newImages...)
					resources = append(resources, models.Resource{
						Kind:      node.(map[string]interface{})["kind"].(string),
						Name:      metadata["name"].(string),
						Namespace: namespace,
						Filename:  fileName,
						Containers: funk.Map(containers, func(c models.Container) string {
							return c.Name
						}).([]string),
					})
				}
			} else {
				metadata := yamlNode["metadata"].(map[string]interface{})
				namespace := ""
				if namespaceObj, ok := metadata["namespace"]; ok {
					namespace = namespaceObj.(string)
				}
				newImages, containers := processYamlNode(yamlNode)
				images = append(images, newImages...)
				resources = append(resources, models.Resource{
					Kind:      kind,
					Name:      metadata["name"].(string),
					Namespace: namespace,
					Filename:  fileName,
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
	owner := trivymodels.Resource{
		Kind: yamlNode["kind"].(string),
		Name: yamlNode["metadata"].(map[string]interface{})["name"].(string),
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
			newContainer := models.Container{
				Image: containerMap["image"].(string),
				Name:  containerMap["name"].(string),
			}
			images = append(images, newContainer)
		}
	}
	return images
}

// GetRepoTags returns the repotags from a tarball of a an image.
func GetRepoTags(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	tarReader := tar.NewReader(f)
	for {
		header, err := tarReader.Next()

		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if header.Name != "manifest.json" {
			continue
		}
		bytes, err := ioutil.ReadAll(tarReader)
		if err != nil {
			return nil, err
		}
		jsonBody := make([]interface{}, 0)
		err = json.Unmarshal(bytes, &jsonBody)
		if err != nil {
			return nil, err
		}
		allRepoTags := make([]string, 0)
		for _, imageDef := range jsonBody {
			repoTags := imageDef.(map[string]interface{})["RepoTags"].([]interface{})
			for _, tag := range repoTags {
				allRepoTags = append(allRepoTags, tag.(string))
			}
		}
		return allRepoTags, nil
	}
	return nil, nil
}

// SendResults sends the results to Insights
func SendResults(reports []models.ReportInfo, resources []models.Resource, configurationObject models.Configuration, token string) (models.ScanResults, error) {
	var b bytes.Buffer
	var results models.ScanResults

	w := multipart.NewWriter(&b)

	for _, report := range reports {
		fw, err := w.CreateFormFile(report.Report, report.Filename)
		if err != nil {
			logrus.Warnf("Unable to create form for %s", report.Report)
			return results, err
		}
		r, err := os.Open(configurationObject.Options.TempFolder + "/" + report.Filename)
		if err != nil {
			logrus.Warnf("Unable to open file for %s", report.Report)
			return results, err
		}
		defer r.Close()
		_, err = io.Copy(fw, r)

		if err != nil {
			logrus.Warnf("Unable to write contents for %s", report.Report)
			return results, err
		}
	}
	w.Close()

	masterHash, err := GetResultsFromCommand("git", "merge-base", "HEAD", "master")
	if err != nil {
		logrus.Warn("Unable to get GIT merge-base")
		return results, err
	}

	currentHash, err := GetResultsFromCommand("git", "rev-parse", "HEAD")
	if err != nil {
		logrus.Warn("Unable to get GIT Hash")
		return results, err
	}

	branchName, err := GetResultsFromCommand("git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		logrus.Warn("Unable to get GIT Branch Name")
		return results, err
	}

	origin := configurationObject.Options.RepositoryName
	if origin == "" {
		origin, err = GetResultsFromCommand("git", "remote", "get-url", "origin")
		if err != nil {
			logrus.Warn("Unable to get GIT Origin")
			return results, err
		}
		if strings.Contains(origin, "@") { // git@github.com URLs are allowed
			originSplit := strings.Split(origin, "@")
			// Take the substring after the last @ to avoid any tokens in an HTTPS URL
			origin = originSplit[len(originSplit)-1]
		} else if strings.Contains(origin, "//") {
			originSplit := strings.Split(origin, "//")
			origin = originSplit[len(originSplit)-1]
		}
		// Remove "******.com:" prefix and ".git" suffix to get clean $org/$repo structure
		if strings.Contains(origin, ":") {
			originSplit := strings.Split(origin, ":")
			origin = originSplit[len(originSplit)-1]
		}
		origin = strings.TrimSuffix(origin, ".git")
	}

	url := fmt.Sprintf("%s/v0/organizations/%s/ci/scan-results", configurationObject.Options.Hostname, configurationObject.Options.Organization)
	req, err := http.NewRequest("POST", url, &b)
	if err != nil {
		logrus.Warn("Unable to create Request")
		return results, err
	}

	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("X-Commit-Hash", currentHash)
	req.Header.Set("X-Branch-Name", branchName)
	req.Header.Set("X-Master-Hash", masterHash)
	req.Header.Set("X-Base-Branch", configurationObject.Options.BaseBranch)
	req.Header.Set("X-Repository-Name", origin)
	req.Header.Set("X-New-AI-Threshold", strconv.Itoa(configurationObject.Options.NewActionItemThreshold))
	req.Header.Set("X-Severity-Threshold", configurationObject.Options.SeverityThreshold)
	req.Header.Set("Authorization", "Bearer "+token)

	for _, report := range reports {
		req.Header.Set("X-Fairwinds-Report-Version-"+report.Report, report.Version)
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
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

	for idx, actionItem := range results.NewActionItems {
		for _, resource := range resources {
			if resource.Kind == actionItem.ResourceKind && resource.Name == actionItem.ResourceName {
				results.NewActionItems[idx].Notes = fmt.Sprintf("Resource was found in file: %s", resource.Filename)
				break
			}
		}
	}

	return results, nil
}

// SaveJUnitFile will save the
func SaveJUnitFile(results models.ScanResults, filename string) error {
	cases := make([]formatter.JUnitTestCase, 0)

	for _, actionItem := range results.NewActionItems {
		cases = append(cases, formatter.JUnitTestCase{
			Name: actionItem.ResourceName + ": " + actionItem.Title,
			Failure: &formatter.JUnitFailure{
				Message:  actionItem.Remediation,
				Contents: fmt.Sprintf("File: %s\nDescription: %s", actionItem.Notes, actionItem.Description),
			},
		})
	}

	for _, actionItem := range results.FixedActionItems {
		cases = append(cases, formatter.JUnitTestCase{
			Name: actionItem.ResourceName + ": " + actionItem.Title,
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

// CheckScore checks if the score meets all of the thresholds.
func CheckScore(results models.ScanResults, configurationObject models.Configuration) error {
	if !results.Pass {
		logrus.Infof("Fairwinds Insights CI check has failed, please fix some Action Items: %v", results.NewActionItems)
		return errors.New(models.ScoreOutOfBoundsMessage)
	}

	return nil
}

// ProcessHelmTemplates turns helm into yaml to be processed by Polaris or the other tools.
func ProcessHelmTemplates(configurationObject models.Configuration, configFolder string) error {
	for _, helmObject := range configurationObject.Manifests.Helm {
		err := util.RunCommand(exec.Command("helm", "dependency", "update", helmObject.Path), "Updating dependencies for "+helmObject.Name)
		if err != nil {
			return err
		}
		params := []string{
			"template", helmObject.Name,
			helmObject.Path,
			"--output-dir",
			configFolder + helmObject.Name,
		}
		valuesFile := helmObject.ValuesFile
		if valuesFile == "" {
			valuesFile = configurationObject.Options.TempFolder + "helmValues.yaml"
			yaml, err := yaml.Marshal(helmObject.Values)
			if err != nil {
				return err
			}
			err = ioutil.WriteFile(valuesFile, yaml, 0644)
		}
		params = append(params, "-f", helmObject.ValuesFile)

		err = util.RunCommand(exec.Command("helm", params...), "Templating: "+helmObject.Name)
		if err != nil {
			return err
		}
	}
	return nil

}

// CopyYaml adds all Yaml found in a given spot into the manifest folder.
func CopyYaml(configurationObject models.Configuration, configFolder string) error {
	for _, path := range configurationObject.Manifests.YamlPaths {
		err := util.RunCommand(exec.Command("cp", "-r", path, configFolder), "Copying yaml file")
		if err != nil {
			return err
		}
	}
	return nil
}
