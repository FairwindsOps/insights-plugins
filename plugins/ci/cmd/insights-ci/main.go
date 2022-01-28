package main

import (
	"context"
	"encoding/json"
	"errors"
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

const maxLinesForPrint = 8

func exitWithError(message string, err error) {
	if err != nil {
		logrus.Fatalf("%s: %s", message, err.Error())
	} else {
		logrus.Fatal(message)
	}
}

func main() {
	err := doNewStuff()
	if err != nil {
		exitWithError("Could not doNewStuff", err) // todo: vitor - change message
	}

	const configFilePath = "./fairwinds-insights.yaml"
	configHandler, err := os.Open(configFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			exitWithError("Please add fairwinds-insights.yaml to the base of your repository.", nil)
		} else {
			exitWithError("Could not open fairwinds-insights.yaml", err)
		}
	}
	configContents, err := ioutil.ReadAll(configHandler)
	if err != nil {
		exitWithError("Could not read fairwinds-insights.yaml", err)
	}
	config := models.Configuration{}
	err = yaml.Unmarshal(configContents, &config)
	if err != nil {
		exitWithError("Could not parse fairwinds-insights.yaml", err)
	}

	config.SetDefaults()
	err = config.CheckForErrors()
	if err != nil {
		exitWithError("Error parsing fairwinds-insights.yaml", err)
	}

	configFolder := config.Options.TempFolder + "/configuration/"
	err = os.MkdirAll(configFolder, 0755)
	if err != nil {
		exitWithError("Could not make directory "+configFolder, err)
	}

	token := strings.TrimSpace(os.Getenv("FAIRWINDS_TOKEN"))
	if token == "" {
		exitWithError("FAIRWINDS_TOKEN environment variable not set", nil)
	}

	if len(config.Manifests.Helm) > 0 {
		err := ci.ProcessHelmTemplates(config.Manifests.Helm, config.Options.TempFolder, configFolder)
		if err != nil {
			exitWithError("Error while processing helm templates", err)
		}
	}
	if len(config.Manifests.YamlPaths) > 0 {
		err := ci.CopyYaml(config, configFolder)
		if err != nil {
			exitWithError("Error while copying YAML files", err)
		}
	}

	// Scan YAML, find all images/kind/etc
	manifestImages, resources, err := ci.GetAllResources(configFolder, config)
	if err != nil {
		exitWithError("Error while extracting images from YAML manifests", err)
	}

	var reports []models.ReportInfo

	// Scan manifests with Polaris
	if *config.Reports.Polaris.Enabled {
		polarisReport, err := getPolarisReport(config, configFolder)
		if err != nil {
			exitWithError("Error while running Polaris", err)
		}
		reports = append(reports, polarisReport)
	}

	if *config.Reports.Trivy.Enabled {
		manifestImagesToScan := manifestImages
		if *config.Reports.Trivy.SkipManifests {
			manifestImagesToScan = []trivymodels.Image{}
		}
		trivyReport, err := getTrivyReport(manifestImagesToScan, config)
		if err != nil {
			exitWithError("Error while running Trivy", err)
		}
		reports = append(reports, trivyReport)
	}

	workloadReport, err := getWorkloadReport(resources, config)
	if err != nil {
		exitWithError("Error while aggregating workloads", err)
	}
	reports = append(reports, workloadReport)

	if *config.Reports.OPA.Enabled {
		opaReport, err := opa.ProcessOPA(context.Background(), config)
		if err != nil {
			exitWithError("Error while running OPA", err)
		}
		reports = append(reports, opaReport)
	}

	if *config.Reports.Pluto.Enabled {
		plutoReport, err := getPlutoReport(config, configFolder)
		if err != nil {
			exitWithError("Error while running Pluto", err)
		}
		reports = append(reports, plutoReport)
	}

	results, err := ci.SendResults(reports, resources, config, token)
	if err != nil {
		exitWithError("Error while sending results back to "+config.Options.Hostname, err)
	}
	fmt.Printf("%d new Action Items:\n", len(results.NewActionItems))
	printActionItems(results.NewActionItems)
	fmt.Printf("%d fixed Action Items:\n", len(results.FixedActionItems))
	printActionItems(results.FixedActionItems)

	if config.Options.JUnitOutput != "" {
		err = ci.SaveJUnitFile(*results, config.Options.JUnitOutput)
		if err != nil {
			exitWithError("Could not save jUnit results", nil)
		}
	}

	if !results.Pass {
		fmt.Printf(
			"\n\nFairwinds Insights checks failed:\n%v\n\nVisit %s/orgs/%s/repositories for more information\n\n",
			err, config.Options.Hostname, config.Options.Organization)
		if config.Options.SetExitCode {
			os.Exit(1)
		}
	} else {
		fmt.Println("\n\nFairwinds Insights checks passed.")
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

type imageCallback func(filename string, sha string, tags []string)

func walkImages(config models.Configuration, cb imageCallback) error {
	err := filepath.Walk(config.Images.FolderName, func(path string, info os.FileInfo, err error) error {
		logrus.Info(path)
		if err != nil {
			logrus.Errorf("Error while walking path %s: %v", path, err)
			return err
		}
		if info.IsDir() {
			return nil
		}
		sha, repoTags, err := ci.GetShaAndRepoTags(path)
		if err != nil {
			return err
		}
		cb(info.Name(), sha, repoTags)
		return nil
	})
	return err
}

func getTrivyReport(manifestImages []trivymodels.Image, configurationObject models.Configuration) (models.ReportInfo, error) {
	trivyReport := models.ReportInfo{
		Report:   "trivy",
		Filename: "trivy.json",
	}

	// Look through image tarballs and mark which ones are already there, by setting image.PullRef
	logrus.Infof("Looking through images in %s", configurationObject.Images.FolderName)
	err := walkImages(configurationObject, func(filename string, sha string, repoTags []string) {
		for idx := range manifestImages {
			if manifestImages[idx].PullRef != "" {
				continue
			}
			for _, tag := range repoTags {
				if tag == manifestImages[idx].Name {
					manifestImages[idx].PullRef = filename
					break
				}
			}
		}
	})
	if err != nil {
		return trivyReport, err
	}

	refLookup := map[string]string{}
	// Download missing images
	for idx := range manifestImages {
		if manifestImages[idx].PullRef != "" {
			continue
		}
		if ref, ok := refLookup[manifestImages[idx].Name]; ok {
			manifestImages[idx].PullRef = ref
			continue
		}
		logrus.Infof("Downloading missing image %s", manifestImages[idx].Name)
		dockerURL := "docker://" + manifestImages[idx].Name
		archiveName := "docker-archive:" + configurationObject.Images.FolderName + strconv.Itoa(idx)
		err := util.RunCommand(exec.Command("skopeo", "copy", dockerURL, archiveName), "pulling "+manifestImages[idx].Name)
		if err != nil {
			return trivyReport, err
		}
		manifestImages[idx].PullRef = strconv.Itoa(idx)
		refLookup[manifestImages[idx].Name] = manifestImages[idx].PullRef
	}

	// Untar images, read manifest.json/RepoTags, match tags to YAML
	logrus.Infof("Extracting details for all images")
	allImages := []trivymodels.Image{}
	err = walkImages(configurationObject, func(filename string, sha string, repoTags []string) {
		logrus.Infof("Getting details for image file %s with SHA %s", filename, sha)

		// If the image was found in a manifest, copy its details over,
		// namely the Owner info (i.e. the deployment or other controller it is associated with)
		var image *trivymodels.Image
		for _, im := range manifestImages {
			if im.PullRef == filename {
				image = &im
				break
			}
		}
		if image == nil {
			image = &trivymodels.Image{
				PullRef: filename,
				Owner: trivymodels.Resource{
					Kind: "Image",
				},
			}
		}

		if len(repoTags) == 0 {
			name := image.Name
			nameParts := strings.Split(name, ":")
			if len(nameParts) > 1 {
				name = nameParts[0]
			}
			if len(name) > 0 {
				image.ID = name + "@" + sha
			} else {
				image.ID = sha
			}
			logrus.Warningf("Could not find repo or tags for %s", filename)
		} else {
			repoAndTag := repoTags[0]
			repo := strings.Split(repoAndTag, ":")[0]
			image.ID = fmt.Sprintf("%s@%s", repo, sha)
			image.Name = repoAndTag
			image.Owner.Name = repo // This name is used for the filename in the Insights UI
		}

		allImages = append(allImages, *image)
	})
	if err != nil {
		return trivyReport, err
	}
	// Scan Images with Trivy
	trivyResults, trivyVersion, err := ci.ScanImagesWithTrivy(allImages, configurationObject)
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
	plutoResults, err := ci.GetResultsFromCommand("pluto", "detect-files", "-d", manifestFolder, "-o", "json", "--ignore-deprecations", "--ignore-removals")
	if err != nil {
		return report, err
	}
	err = ioutil.WriteFile(configurationObject.Options.TempFolder+"/"+report.Filename, []byte(plutoResults), 0644)
	if err != nil {
		return report, err
	}
	report.Version = os.Getenv("plutoVersion")
	return report, nil
}

func doNewStuff() error {
	repoName := strings.TrimSpace(os.Getenv("REPOSITORY_NAME"))
	if repoName == "" {
		// no repository name set, do normal flow
		return nil
	}

	token := strings.TrimSpace(os.Getenv("FAIRWINDS_TOKEN"))
	if token == "" {
		return errors.New("FAIRWINDS_TOKEN environment variable not set")
	}

	branch := strings.TrimSpace(os.Getenv("BRANCH"))
	if branch == "" {
		return errors.New("BRANCH environment variable not set")
	}

	accessToken := strings.TrimSpace(os.Getenv("ACCCESS_TOKEN"))
	if accessToken == "" {
		return errors.New("ACCCESS_TOKEN environment variable not set")
	}

	if strings.TrimSpace(os.Getenv("IMAGE_VERSION")) == "" {
		return errors.New("IMAGE_VERSION environment variable not set")
	}

	logrus.Infof("all required variables are set")

	err := util.RunCommandInDir("app/repository", exec.Command("git", "clone", "--branch", branch, fmt.Sprintf("https://x-access-token:%s@github.com/%s.git", accessToken, repoName)), "Cloning github repository")
	if err != nil {
		exitWithError("unable to clone repository", err)
	}
	logrus.Infof("git clone executed successfully")

	return nil
}
