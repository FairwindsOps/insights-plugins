package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
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

	"github.com/fairwindsops/insights-plugins/trivy/pkg/image"
	"github.com/fairwindsops/insights-plugins/trivy/pkg/models"
	"github.com/fairwindsops/insights-plugins/trivy/pkg/util"
	"github.com/sirupsen/logrus"
	"github.com/thoas/go-funk"
	"gopkg.in/yaml.v3"
)

type configuration struct {
	Images    folderConfig `yaml:"images"`
	Manifests folderConfig `yaml:"manifests"`
	Options   optionConfig `yaml:"options"`
}

type optionConfig struct {
	Fail                 bool    `yaml:"fail"`
	ScoreThreshold       float64 `yaml:"scoreThreshold"`
	ScoreChangeThreshold float64 `yaml:"scoreChangeThreshold"`
	TempFolder           string  `yaml:"tempFolder"`
	Hostname             string  `yaml:"hostname"`
	Organization         string  `yaml:"organization"`
}

type folderConfig struct {
	FolderName string   `yaml:"folder"`
	Commands   []string `yaml:"cmd"`
}

func main() {
	const configFile = "./insights-config"
	configurationObject := configuration{
		Images: folderConfig{
			FolderName: "./insights/images",
		},
		Manifests: folderConfig{
			FolderName: "./insights/manifests",
		},
		Options: optionConfig{
			ScoreThreshold:       0.6,
			ScoreChangeThreshold: 0.4,
			TempFolder:           "./insights/temp",
		},
	}
	configHandler, err := os.Open(configFile)
	if err == nil {
		configContents, err := ioutil.ReadAll(configHandler)
		if err != nil {
			panic(err)
		}
		err = yaml.Unmarshal(configContents, &configurationObject)
		if err != nil {
			panic(err)
		}
	} else if !os.IsNotExist(err) {
		panic(err)
	}

	// Parse out config

	configFolder := configurationObject.Manifests.FolderName + "/"
	imageFolder := configurationObject.Images.FolderName + "/"
	token := os.Getenv("FAIRWINDS_TOKEN")
	// Scan YAML, find all images/kind/etc
	images := make([]models.Image, 0)
	err = filepath.Walk(configFolder, func(path string, info os.FileInfo, err error) error {
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
	if err != nil {
		panic(err)
	}
	// Untar images, read manifest.json/RepoTags, match tags to YAML
	err = filepath.Walk(imageFolder, func(path string, info os.FileInfo, err error) error {
		logrus.Info(path)
		if info.IsDir() {
			return nil
		}
		repoTags, err := getRepoTags(path)
		if err != nil {
			return err
		}
		for idx, currentImage := range images {
			if currentImage.PullRef != "" {
				continue
			}
			for _, tag := range repoTags {
				logrus.Info(tag, currentImage.Name)
				if tag == currentImage.Name {
					images[idx].PullRef = info.Name()
					break
				}
			}
		}
		return nil
	})
	if err != nil {
		panic(err)
	}

	// Download missing images
	for idx, currentImage := range images {
		if currentImage.PullRef != "" {
			continue
		}

		err := util.RunCommand(exec.Command("skopeo", "copy", "docker://"+currentImage.Name, "docker-archive:"+imageFolder+strconv.Itoa(idx)), "pulling "+currentImage.Name)
		if err != nil {
			panic(err)
		}
		images[idx].PullRef = strconv.Itoa(idx)
	}
	// Scan Images with Trivy
	err = util.RunCommand(exec.Command("trivy", "--download-db-only"), "downloading trivy database")
	if err != nil {
		panic(err)
	}
	reportByRef := funk.Map(images, func(currentImage models.Image) (string, []models.VulnerabilityList) {
		results, err := image.ScanImageFile(imageFolder+currentImage.PullRef, currentImage.PullRef, configurationObject.Options.TempFolder)
		if err != nil {
			panic(err)
		}
		return currentImage.PullRef, results
	}).(map[string][]models.VulnerabilityList)
	allReports := image.ConvertTrivyResultsToImageReport(images, reportByRef)
	// Collate results
	results := image.Minimize(allReports, models.MinimizedReport{Images: make([]models.ImageDetailsWithRefs, 0), Vulnerabilities: map[string]models.VulnerabilityDetails{}})
	trivyResults, err := json.Marshal(results)
	if err != nil {
		panic(err)
	}
	// Scan with Polaris
	err = util.RunCommand(exec.Command("polaris", "-audit", "-audit-path", configFolder, "-output-file", configurationObject.Options.TempFolder+"/polaris.json"), "Audit with Polaris")

	// Send Results up
	trivyVersion, err := getResultsFromCommand("trivy", "--version")
	if err != nil {
		panic(err)
	}
	trivyVersion = strings.Split(strings.Split(trivyVersion, "\n")[0], " ")[1]

	polarisVersion, err := getResultsFromCommand("polaris", "--version")
	if err != nil {
		panic(err)
	}
	polarisVersion = strings.Split(polarisVersion, " ")[2]

	err = sendResults(trivyResults, trivyVersion, polarisVersion, configurationObject, token)
	if err != nil {
		panic(err)
	}
}

func getResultsFromCommand(command string, args ...string) (string, error) {
	bytes, err := exec.Command(command, args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(bytes)), err
}

func sendResults(trivyResults []byte, trivyVersion string, polarisVersion string, configurationObject configuration, token string) error {
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

	masterHash, err := getResultsFromCommand("git", "merge-base", "HEAD", "master")
	if err != nil {
		logrus.Warn("Unable to get GIT merge-base")
		return err
	}

	currentHash, err := getResultsFromCommand("git", "rev-parse", "HEAD")
	if err != nil {
		logrus.Warn("Unable to get GIT Hash")
		return err
	}

	branchName, err := getResultsFromCommand("git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		logrus.Warn("Unable to get GIT Branch Name")
		return err
	}

	origin, err := getResultsFromCommand("git", "remote", "get-url", "origin")
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

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Warn("Unable to read results")
		return err
	}
	logrus.Info(body)
	return nil
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

func getRepoTags(path string) ([]string, error) {
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
