package main

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fairwindsops/insights-plugins/ci/pkg/ci"
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
	imageFolder := configurationObject.Images.FolderName
	token := strings.TrimSpace(os.Getenv("FAIRWINDS_TOKEN"))
	// Scan YAML, find all images/kind/etc
	images, err := ci.GetImagesFromManifest(configFolder)
	if err != nil {
		panic(err)
	}
	// Untar images, read manifest.json/RepoTags, match tags to YAML
	err = filepath.Walk(imageFolder, func(path string, info os.FileInfo, err error) error {
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
			images = append(images, models.Image { 
				PullRef: info.Name()
			})
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
	trivyResults, trivyVersion, err := ci.ScanImagesWithTrivy(images, configurationObject)
	if err != nil {
		panic(err)
	}
	// Scan with Polaris
	err = util.RunCommand(exec.Command("polaris", "-audit", "-audit-path", configFolder, "-output-file", configurationObject.Options.TempFolder+"/polaris.json"), "Audit with Polaris")

	// Send Results up
	if err != nil {
		panic(err)
	}

	polarisVersion, err := ci.GetResultsFromCommand("polaris", "--version")
	if err != nil {
		panic(err)
	}
	polarisVersion = strings.Split(polarisVersion, " ")[2]

	err = ci.SendResults(trivyResults, trivyVersion, polarisVersion, configurationObject, token)
	if err != nil {
		if err.Error() == ci.ScoreOutOfBoundsMessage && !configurationObject.Options.Fail {
			return

		}
		panic(err)
	}
}
