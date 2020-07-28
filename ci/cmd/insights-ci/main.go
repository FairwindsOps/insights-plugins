package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fairwindsops/insights-plugins/ci/pkg/ci"
	"github.com/fairwindsops/insights-plugins/trivy/pkg/models"
	"github.com/fairwindsops/insights-plugins/ci/pkg/util"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

const workloadsReportVersion = "0.1.0"

func maybeAddSlash(input string) string {
	if strings.HasSuffix(input, "/") {
		return input
	}
	return input + "/"
}

func main() {
	const configFile = "./fairwinds-insights.yaml"
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
	configurationObject.Options.TempFolder = maybeAddSlash(configurationObject.Options.TempFolder)
	configurationObject.Images.FolderName = maybeAddSlash(configurationObject.Images.FolderName)

	configFolder := configurationObject.Options.TempFolder + "/configuration/"
	err = os.Mkdir(configFolder, 0644)
	if err != nil {
		panic(err)
	}
	token := strings.TrimSpace(os.Getenv("FAIRWINDS_TOKEN"))
	if len(configurationObject.Manifests.Helm) > 0 {
		err := ci.ProcessHelmTemplates(configurationObject, configFolder)
		if err != nil {
			panic(err)
		}
	}
	if len(configurationObject.Manifests.YamlPaths) > 0 {
		err := ci.CopyYaml(configurationObject, configFolder)
		if err != nil {
			panic(err)
		}
	}
	// Scan YAML, find all images/kind/etc
	images, resources, err := ci.GetImagesFromManifest(configFolder)
	if err != nil {
		panic(err)
	}

	// Scan manifests with Polaris
	polarisReport, err := getPolarisReport(configurationObject, configFolder)
	if err != nil {
		panic(err)
	}

	trivyReport, err := getTrivyReport(images, configurationObject)
	if err != nil {
		panic(err)
	}

	workloadReport, err := getWorkloadReport(resources, configurationObject)
	if err != nil {
		panic(err)
	}

	results, err := ci.SendResults([]ci.ReportInfo{trivyReport, polarisReport, workloadReport}, resources, configurationObject, token)
	if err != nil {
		panic(err)
	}
	logrus.Infof("New Action Item Count: %d Fixed Action Item Count: %d", len(results.NewActionItems), len(results.FixedActionItems))

	if configurationObject.Options.JUnitOutput != "" {
		err = ci.SaveJUnitFile(results, configurationObject)
		if err != nil {
			panic(err)
		}
	}
	if configurationObject.Options.SetExitCode {
		err = ci.CheckScore(results, configurationObject)
		if err != nil {
			panic(err)
		}
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
	err = ioutil.WriteFile(configurationObject.Options.TempFolder+"/"+trivyReport.Filename, trivyResults, 0644)
	if err != nil {
		return trivyReport, err
	}

	trivyReport.Version = trivyVersion
	return trivyReport, nil
}

func getWorkloadReport(resources []ci.Resource, configurationObject ci.Configuration) (ci.ReportInfo, error) {
	workloadsReport := ci.ReportInfo{
		Report:   "scan-workloads",
		Filename: "scan-workloads.json",
	}
	resourceBytes, err := json.Marshal(map[string]interface{}{"Resources": resources})
	if err != nil {
		return workloadsReport, err
	}
	err = ioutil.WriteFile(configurationObject.Options.TempFolder+"/"+workloadsReport.Filename, resourceBytes, 0644)
	if err != nil {
		return workloadsReport, err
	}

	workloadsReport.Version = workloadsReportVersion
	return workloadsReport, nil
}

func getPolarisReport(configurationObject ci.Configuration, manifestFolder string) (ci.ReportInfo, error) {
	report := ci.ReportInfo{
		Report:   "polaris",
		Filename: "polaris.json",
	}
	// Scan with Polaris
	err := util.RunCommand(exec.Command("polaris", "audit", "--audit-path", manifestFolder, "--output-file", configurationObject.Options.TempFolder+"/"+report.Filename), "Audit with Polaris")
	if err != nil {
		return report, err
	}
	polarisVersion, err := ci.GetResultsFromCommand("polaris", "version")
	if err != nil {
		return report, err
	}
	report.Version = strings.Split(polarisVersion, ":")[1]
	return report, nil
}
