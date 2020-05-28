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
	"strings"

	"github.com/fairwindsops/insights-plugins/trivy/pkg/models"
	"github.com/jstemmer/go-junit-report/formatter"
	"github.com/sirupsen/logrus"
	"github.com/thoas/go-funk"
	"gopkg.in/yaml.v3"
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
func GetImagesFromManifest(configFolder string) ([]models.Image, error) {
	images := make([]models.Image, 0)
	err := filepath.Walk(configFolder, func(path string, info os.FileInfo, err error) error {
		if strings.HasSuffix(info.Name(), ".yaml") {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			decoder := yaml.NewDecoder(file)
			for {
				yamlNode := make(map[string]interface{})

				err = decoder.Decode(&yamlNode)
				if err != nil {
					if err != io.EOF {
						return err
					}
					break

				}
				kind := yamlNode["kind"].(string)
				if kind == "list" {
					nodes := yamlNode["items"].([]interface{})
					for _, node := range nodes {
						images = append(images, processYamlNode(node.(map[string]interface{}))...)
					}
				} else {
					images = append(images, processYamlNode(yamlNode)...)
				}

			}

		}
		return nil
	})
	return images, err
}

func processYamlNode(yamlNode map[string]interface{}) []models.Image {
	owner := models.Resource{
		Kind: yamlNode["kind"].(string),
		Name: yamlNode["metadata"].(map[string]interface{})["name"].(string),
	}
	podSpec := GetPodSpec(yamlNode)
	images := getImages(podSpec.(map[string]interface{}))
	return funk.Map(images, func(s string) models.Image {
		return models.Image{
			Name:  s,
			Owner: owner,
		}
	}).([]models.Image)
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

func getImages(podSpec map[string]interface{}) []string {
	images := make([]string, 0)
	for _, field := range containerSpecFields {
		containerField, ok := podSpec[field]
		if !ok {
			continue
		}
		containers := containerField.([]interface{})
		for _, container := range containers {
			images = append(images, container.(map[string]interface{})["image"].(string))
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
func SendResults(trivyResults []byte, trivyVersion string, polarisVersion string, configurationObject Configuration, token string) error {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	var fw io.Writer
	fw, err := w.CreateFormFile("trivy", "trivy.json")
	if err != nil {
		logrus.Warn("Unable to create form for Trivy")
		return err
	}
	_, err = fw.Write(trivyResults)
	if err != nil {
		logrus.Warn("Unable to write contents for Trivy")
		return err
	}

	fw, err = w.CreateFormFile("polaris", "polaris.json")
	if err != nil {
		logrus.Warn("Unable to create form for Polaris")
		return err
	}
	r, err := os.Open(configurationObject.Options.TempFolder + "/polaris.json")
	if err != nil {
		logrus.Warn("Unable to open file for Polaris")
		return err
	}
	defer r.Close()
	_, err = io.Copy(fw, r)

	if err != nil {
		logrus.Warn("Unable to write contents for Polaris")
		return err
	}

	w.Close()

	masterHash, err := GetResultsFromCommand("git", "merge-base", "HEAD", "master")
	if err != nil {
		logrus.Warn("Unable to get GIT merge-base")
		return err
	}

	currentHash, err := GetResultsFromCommand("git", "rev-parse", "HEAD")
	if err != nil {
		logrus.Warn("Unable to get GIT Hash")
		return err
	}

	branchName, err := GetResultsFromCommand("git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		logrus.Warn("Unable to get GIT Branch Name")
		return err
	}

	origin, err := GetResultsFromCommand("git", "remote", "get-url", "origin")
	if err != nil {
		logrus.Warn("Unable to get GIT Origin")
		return err
	}

	headers := map[string]string{
		"Content-Type":                       w.FormDataContentType(),
		"X-Fairwinds-Report-Version-Trivy":   trivyVersion,
		"X-Fairwinds-Report-Version-Polaris": polarisVersion,
		"X-Commit-Hash":                      currentHash,
		"X-Branch-Name":                      branchName,
		"X-Master-Hash":                      masterHash,
		"X-Repository-Name":                  origin,
		"Authorization":                      "Bearer " + token,
	}

	url := fmt.Sprintf("%s/v0/organizations/%s/ci/scan-results", configurationObject.Options.Hostname, configurationObject.Options.Organization)
	req, err := http.NewRequest("POST", url, &b)
	if err != nil {
		logrus.Warn("Unable to create Request")
		return err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		logrus.Warn("Unable to Post results to Insights")
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Invalid status code: %d", resp.StatusCode)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Warn("Unable to read results")
		return err
	}
	var results ScanResults
	err = json.Unmarshal(body, &results)
	if err != nil {
		return err
	}
	if configurationObject.Options.JUnitOutput != "" {
		cases := make([]formatter.JUnitTestCase, 0)

		for _, actionItem := range results.ActionItems {
			cases = append(cases, formatter.JUnitTestCase{
				Name: actionItem.ResourceName + ": " + actionItem.Title,
				Failure: &formatter.JUnitFailure{
					Message:  actionItem.Remediation,
					Contents: actionItem.Description,
				},
			})
		}

		testSuites := formatter.JUnitTestSuites{
			Suites: []formatter.JUnitTestSuite{
				{
					Tests:     len(results.ActionItems),
					TestCases: cases,
				},
			},
		}

		xmlBytes, err := xml.MarshalIndent(testSuites, "", "\t")
		if err != nil {
			return err
		}
		xmlBytes = append([]byte(xml.Header), xmlBytes...)
		err = ioutil.WriteFile(configurationObject.Options.JUnitOutput, xmlBytes, 0644)
		if err != nil {
			return err
		}
	}
	logrus.Infof("Score of %f with a baseline of %f", results.Score, results.BaselineScore)
	if configurationObject.Options.ScoreThreshold < results.Score || configurationObject.Options.ScoreChangeThreshold < results.BaselineScore-results.Score {
		logrus.Infof("Score is out of bounds, please fix some Action Items: %v", results.ActionItems)
		return errors.New(ScoreOutOfBoundsMessage)
	}

	return nil
}
