package main

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fairwindsops/insights-plugins/ci/pkg/ci"
	"github.com/fairwindsops/insights-plugins/trivy/pkg/models"
	"github.com/fairwindsops/insights-plugins/trivy/pkg/util"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

func maybeAddSlash(input string) string {
	if strings.HasSuffix(input, "/") {
		return input
	}
	return input + "/"
}

func main() {
	const configFile = "./insights-config"
	configurationObject := ci.GetDefaultConfig()
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
	configurationObject.Manifests.FolderName = maybeAddSlash(configurationObject.Manifests.FolderName)
	configurationObject.Images.FolderName = maybeAddSlash(configurationObject.Images.FolderName)
	// Parse out config

	configFolder := configurationObject.Manifests.FolderName
	token := strings.TrimSpace(os.Getenv("FAIRWINDS_TOKEN"))
	// Scan YAML, find all images/kind/etc
	images, resources, err := ci.GetImagesFromManifest(configFolder)
	if err != nil {
		panic(err)
	}

	// Scan manifests with Polaris
	polarisReport, err := getPolarisReport(configurationObject)
	if err != nil {
		panic(err)
	}
	// Send Results up
	trivyReport, err := getTrivyReport(images, configurationObject)
	if err != nil {
		panic(err)
	}

	err = ci.SendResults([]ci.ReportInfo{trivyReport, polarisReport}, resources, configurationObject, token)
	if err != nil {
		if err.Error() == ci.ScoreOutOfBoundsMessage && !configurationObject.Options.Fail {
			return

		}
		panic(err)
	}
}

func getTrivyReport(images []models.Image, configurationObject ci.Configuration) (ci.ReportInfo, error) {
	trivyReport := ci.ReportInfo{
		Report:   "trivy",
		Filename: "trivy.json",
	}
	// Untar images, read manifest.json/RepoTags, match tags to YAML
	err := filepath.Walk(configurationObject.Images.FolderName, func(path string, info os.FileInfo, err error) error {
		logrus.Info(path)
		if info.IsDir() {
			return nil
		}
		repoTags, err := ci.GetRepoTags(path)
		if err != nil {
			return err
		}
		matchedImage := false
		for idx, currentImage := range images {
			if currentImage.PullRef != "" {
				continue
			}
			for _, tag := range repoTags {
				logrus.Info(tag, currentImage.Name)
				if tag == currentImage.Name {
					images[idx].PullRef = info.Name()
					matchedImage = true
					break
				}
			}
		}
		if !matchedImage {
			images = append(images, models.Image{
				PullRef: info.Name(),
			})
		}
		return nil
	})
	if err != nil {
		return trivyReport, err
	}

	// Download missing images
	for idx, currentImage := range images {
		if currentImage.PullRef != "" {
			continue
		}

		err := util.RunCommand(exec.Command("skopeo", "copy", "docker://"+currentImage.Name, "docker-archive:"+configurationObject.Images.FolderName+strconv.Itoa(idx)), "pulling "+currentImage.Name)
		if err != nil {
			return trivyReport, err
		}
		images[idx].PullRef = strconv.Itoa(idx)
	}
	// Scan Images with Trivy
	trivyResults, trivyVersion, err := ci.ScanImagesWithTrivy(images, configurationObject)
	if err != nil {
		return trivyReport, err
	}
	err = ioutil.WriteFile(configurationObject.Options.TempFolder+"/trivy.json", trivyResults, 0644)
	if err != nil {
		return trivyReport, err
	}

	trivyReport.Version = trivyVersion
	return trivyReport, nil
}

func getPolarisReport(configurationObject ci.Configuration) (ci.ReportInfo, error) {
	report := ci.ReportInfo{
		Report:   "polaris",
		Filename: "polaris.json",
	}
	// Scan with Polaris
	err := util.RunCommand(exec.Command("polaris", "-audit", "-audit-path", configurationObject.Manifests.FolderName, "-output-file", configurationObject.Options.TempFolder+"/polaris.json"), "Audit with Polaris")
	if err != nil {
		return report, err
	}
	polarisVersion, err := ci.GetResultsFromCommand("polaris", "--version")
	if err != nil {
		return report, err
	}
	report.Version = strings.Split(polarisVersion, " ")[2]
	return report, nil
}
