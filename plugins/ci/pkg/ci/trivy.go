package ci

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/image"
	trivymodels "github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/models"
	"github.com/sirupsen/logrus"

	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/commands"
	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/models"
)

func (ci *CIScan) GetTrivyReport(manifestImages []trivymodels.Image) (models.ReportInfo, error) {
	trivyReport := models.ReportInfo{
		Report:   "trivy",
		Filename: "trivy.json",
	}

	// Look through image tarballs and mark which ones are already there, by setting image.PullRef
	logrus.Infof("Looking through images in %s", ci.config.Images.FolderName)
	err := walkImages(ci.config, func(filename string, sha string, repoTags []string) {
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
		archiveName := "docker-archive:" + ci.config.Images.FolderName + strconv.Itoa(idx)
		_, err := commands.ExecWithMessage(exec.Command("skopeo", "copy", dockerURL, archiveName), "pulling "+manifestImages[idx].Name)
		if err != nil {
			return trivyReport, err
		}
		manifestImages[idx].PullRef = strconv.Itoa(idx)
		refLookup[manifestImages[idx].Name] = manifestImages[idx].PullRef
	}

	// Untar images, read manifest.json/RepoTags, match tags to YAML
	logrus.Infof("Extracting details for all images")
	allImages := []trivymodels.Image{}
	err = walkImages(ci.config, func(filename string, sha string, repoTags []string) {
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
	trivyResults, trivyVersion, err := scanImagesWithTrivy(allImages)
	if err != nil {
		return trivyReport, err
	}
	err = ioutil.WriteFile(filepath.Join(ci.config.Options.TempFolder, trivyReport.Filename), trivyResults, 0644)
	if err != nil {
		return trivyReport, err
	}

	trivyReport.Version = trivyVersion
	return trivyReport, nil
}

type imageCallback func(filename string, sha string, tags []string)

func walkImages(config *models.Configuration, cb imageCallback) error {
	err := filepath.Walk(config.Images.FolderName, func(path string, info os.FileInfo, err error) error {
		logrus.Info(path)
		if err != nil {
			logrus.Errorf("Error while walking path %s: %v", path, err)
			return err
		}
		if info.IsDir() {
			return nil
		}
		sha, repoTags, err := getShaAndRepoTags(path)
		if err != nil {
			return err
		}
		cb(info.Name(), sha, repoTags)
		return nil
	})
	return err
}

// scanImagesWithTrivy scans the images and returns a Trivy report ready to send to Insights.
func scanImagesWithTrivy(images []trivymodels.Image) ([]byte, string, error) {
	_, err := commands.ExecWithMessage(exec.Command("trivy", "image", "--download-db-only"), "downloading trivy database")
	if err != nil {
		return nil, "", err
	}
	reportByRef := map[string]*trivymodels.TrivyResults{}
	for _, currentImage := range images {
		_, ok := reportByRef[currentImage.PullRef]
		if ok {
			continue
		}
		logrus.Infof("Scanning %s with pullRref %s", currentImage.Name, currentImage.PullRef)
		results, err := image.ScanImage("", currentImage.PullRef)
		if err != nil {
			return nil, "", err
		}
		reportByRef[currentImage.PullRef] = results
	}

	allReports := image.ConvertTrivyResultsToImageReport(images, reportByRef)
	// Collate results
	results := image.Minimize(allReports, trivymodels.MinimizedReport{Images: make([]trivymodels.ImageDetailsWithRefs, 0), Vulnerabilities: map[string]trivymodels.VulnerabilityDetails{}})
	trivyResults, err := json.Marshal(results)
	if err != nil {
		return nil, "", err
	}

	trivyVersion, err := commands.Exec("trivy", "--version")
	if err != nil {
		return nil, "", err
	}
	trivyVersion = strings.Split(strings.Split(trivyVersion, "\n")[0], " ")[1]
	return trivyResults, trivyVersion, nil
}

// getShaAndRepoTags returns the SHA and repo-tags from a tarball of a an image.
func getShaAndRepoTags(path string) (string, []string, error) {
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
		sha, err := getImageSha(path, configFileName)
		if err != nil {
			return "", nil, err
		}
		return sha, allRepoTags, nil
	}
	return "", nil, err
}

// getImageSha returns the sha from a tarball of a an image.
func getImageSha(path string, configFileName string) (string, error) {
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
		imageSha := jsonBody.(map[string]interface{})["config"].(map[string]interface{})["Image"]

		if imageSha != nil {
			sha, ok := imageSha.(string)
			if !ok {
				return "", nil
			}
			return sha, nil
		}
	}
	return "", nil
}

func (ci *CIScan) TrivyEnabled() bool {
	return *ci.config.Reports.Trivy.Enabled
}

func (ci *CIScan) SkipTrivyManifests() bool {
	return *ci.config.Reports.Trivy.SkipManifests
}
