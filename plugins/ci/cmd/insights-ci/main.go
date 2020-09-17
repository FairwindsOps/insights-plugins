package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	trivymodels "github.com/fairwindsops/insights-plugins/trivy/pkg/models"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/fairwindsops/insights-plugins/ci/pkg/ci"
	"github.com/fairwindsops/insights-plugins/ci/pkg/models"
	"github.com/fairwindsops/insights-plugins/ci/pkg/opa"
	"github.com/fairwindsops/insights-plugins/ci/pkg/util"
)

const workloadsReportVersion = "0.1.0"

func exitWithError(message string, err error) {
	if err != nil {
		logrus.Fatalf("%s: %s", message, err.Error())
	} else {
		logrus.Fatal(message)
	}
}

func main() {
	ctx := context.Background()
	const configFile = "./fairwinds-insights.yaml"
	configurationObject := models.Configuration{}
	configHandler, err := os.Open(configFile)
	if err == nil {
		configContents, err := ioutil.ReadAll(configHandler)
		if err != nil {
			exitWithError("Could not read fairwinds-insights.yaml", err)
		}
		err = yaml.Unmarshal(configContents, &configurationObject)
		if err != nil {
			exitWithError("Could not parse fairwinds-insights.yaml", err)
		}
	} else if !os.IsNotExist(err) {
		exitWithError("Could not open fairwinds-insights.yaml", err)
	} else {
		exitWithError("Please add fairwinds-insights.yaml to the base of your repository.", nil)
	}
	configurationObject.SetDefaults()
	err = configurationObject.CheckForErrors()
	if err != nil {
		exitWithError("Error parsing fairwinds-insights.yaml", err)
	}

	configFolder := configurationObject.Options.TempFolder + "/configuration/"
	err = os.MkdirAll(configFolder, 0644)
	if err != nil {
		exitWithError("Could not make directory "+configFolder, nil)
	}

	token := strings.TrimSpace(os.Getenv("FAIRWINDS_TOKEN"))
	if token == "" {
		exitWithError("FAIRWINDS_TOKEN environment variable not set", nil)
	}

	if len(configurationObject.Manifests.Helm) > 0 {
		err := ci.ProcessHelmTemplates(configurationObject, configFolder)
		if err != nil {
			exitWithError("Error while processing helm templates", err)
		}
	}
	if len(configurationObject.Manifests.YamlPaths) > 0 {
		err := ci.CopyYaml(configurationObject, configFolder)
		if err != nil {
			exitWithError("Error while copying YAML files", err)
		}
	}

	// Scan YAML, find all images/kind/etc
	images, resources, err := ci.GetImagesFromManifest(configFolder)
	if err != nil {
		exitWithError("Error while extracting images from YAML manifests", err)
	}

	// Scan manifests with Polaris
	polarisReport, err := getPolarisReport(configurationObject, configFolder)
	if err != nil {
		exitWithError("Error while running Polaris", err)
	}

	trivyReport, err := getTrivyReport(images, configurationObject)
	if err != nil {
		exitWithError("Error while running Trivy", err)
	}

	workloadReport, err := getWorkloadReport(resources, configurationObject)
	if err != nil {
		exitWithError("Error while aggregating workloads", err)
	}

	opaReport, err := opa.ProcessOPA(ctx, configurationObject)
	if err != nil {
		exitWithError("Error while running OPA", err)
	}

	plutoReport, err := getPlutoReport(configurationObject, configFolder)
	if err != nil {
		exitWithError("Error while running Plutoo", err)
	}

	results, err := ci.SendResults([]models.ReportInfo{trivyReport, polarisReport, workloadReport, opaReport, plutoReport}, resources, configurationObject, token)
	if err != nil {
		exitWithError("Error while sending results back to "+configurationObject.Options.Hostname, err)
	}
	logrus.Infof("New Action Item Count: %d Fixed Action Item Count: %d", len(results.NewActionItems), len(results.FixedActionItems))

	if configurationObject.Options.JUnitOutput != "" {
		err = ci.SaveJUnitFile(results, configurationObject.Options.JUnitOutput)
		if err != nil {
			exitWithError("Could not save jUnit results", nil)
		}
	}
	err = ci.CheckScore(results, configurationObject)
	if err != nil {
		fmt.Printf(
			"\n\nFairwinds Insights checks failed:\n%v\n\nVisit %s/orgs/%s/repositories for more information\n\n",
			err, configurationObject.Options.Hostname, configurationObject.Options.Organization)
		if configurationObject.Options.SetExitCode {
			os.Exit(1)
		}
	}
}

func getTrivyReport(images []trivymodels.Image, configurationObject models.Configuration) (models.ReportInfo, error) {
	trivyReport := models.ReportInfo{
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
		if !matchedImage && len(repoTags) > 0 {
			imageRepo := repoTags[0]
			imageRepo = strings.Split(imageRepo, ":")[0]
			images = append(images, trivymodels.Image{
				Name:    repoTags[0], // This name is used in the title
				PullRef: info.Name(),
				Owner: trivymodels.Resource{
					Kind: "Image",
					Name: imageRepo, // This name is used for the filename
				},
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

func getWorkloadReport(resources []models.Resource, configurationObject models.Configuration) (models.ReportInfo, error) {
	workloadsReport := models.ReportInfo{
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

func getPolarisReport(configurationObject models.Configuration, manifestFolder string) (models.ReportInfo, error) {
	report := models.ReportInfo{
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

func getPlutoReport(configurationObject models.Configuration, manifestFolder string) (models.ReportInfo, error) {
	report := models.ReportInfo{
		Report:   "pluto",
		Filename: "pluto.json",
	}
	// Scan with Pluto
	plutoResults, err := ci.GetResultsFromCommand("pluto", "detect-files", "-d", manifestFolder, "-o", "json")
	if err != nil {
		return report, err
	}
	err = ioutil.WriteFile(configurationObject.Options.TempFolder+"/"+report.Filename, []byte(plutoResults), 0644)
	report.Version = os.Getenv("plutoVersion")
	return report, nil
}
