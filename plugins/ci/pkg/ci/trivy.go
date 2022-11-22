package ci

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/image"
	trivymodels "github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/util"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"

	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/commands"
	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/models"
)

func (ci *CIScan) GetTrivyReport(dockerImages []trivymodels.DockerImage, manifestImages []trivymodels.Image) (*models.ReportInfo, error) {
	err := updatePullRef(ci.config.Images.FolderName, dockerImages, manifestImages)
	if err != nil {
		return nil, err
	}

	err = downloadMissingImages(ci.config.Images.FolderName, dockerImages, manifestImages, ci.config.Options.RegistryCredentials)
	if err != nil {
		return nil, err
	}

	allImages, err := mergeImages(ci.config.Images.FolderName, manifestImages)
	if err != nil {
		return nil, err
	}

	trivyResults, trivyVersion, err := scanImagesWithTrivy(allImages, *ci.config)
	if err != nil {
		return nil, err
	}

	filename := "trivy.json"
	err = os.WriteFile(filepath.Join(ci.config.Options.TempFolder, filename), trivyResults, 0644)
	if err != nil {
		return nil, err
	}

	return &models.ReportInfo{
		Report:   "trivy",
		Filename: filename,
		Version:  trivyVersion,
	}, nil
}

// filename -> postgres151bullseye.tar
// sha 			-> sha256:5918a4f7e04aed7ac69d2b03b9b91345556db38709f9d6354056e3fdd9a8c02f
// repoTags -> []string{"postgres:15.1-bullseye"}
type imageCallback func(filename string, sha string, tags []string)

func walkImages(folderPath string, cb imageCallback) error {
	err := filepath.Walk(folderPath, func(path string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			logrus.Errorf("Error while walking path %s: %v", path, err)
			return err
		}
		if fileInfo.IsDir() {
			return nil
		}
		sha, repoTags, err := getShaAndRepoTags(path)
		if err != nil {
			return err
		}
		cb(fileInfo.Name(), sha, repoTags)
		return nil
	})
	return err
}

// updatePullRef looks through image tarballs and mark which ones are already there, by setting image.PullRef
func updatePullRef(folderPath string, dockerImages []trivymodels.DockerImage, manifestImages []trivymodels.Image) error {
	logrus.Infof("Looking through images in %s", folderPath)
	return walkImages(folderPath, func(filename string, sha string, repoTags []string) {
		for _, image := range manifestImages {
			if slices.Contains(repoTags, image.Name) {
				// already downloaded
				logrus.Infof("image (manifest) %s already downloaded", image.Name)
				image.PullRef = filename
			}
		}

		for _, image := range dockerImages {
			if slices.Contains(repoTags, image.Name) {
				// already downloaded
				logrus.Infof("image (docker) %s already downloaded", image.Name)
				image.PullRef = filename
			}
		}
	})
}

func downloadMissingImages(folderPath string, dockerImages []trivymodels.DockerImage, manifestImages []trivymodels.Image, registryCredentials models.RegistryCredentials) error {
	refLookup := map[string]string{} // postgres:15.1-bullseye -> postgres151bullseye
	// Download missing images
	for _, image := range manifestImages {
		if image.PullRef != "" {
			continue
		}
		if ref, ok := refLookup[image.Name]; ok {
			image.PullRef = ref
			continue
		}
		rc := registryCredentials.FindCredentialForImage(image.Name)
		_, err := downloadImageViaSkopeo(commands.ExecWithMessage, folderPath, image.Name, rc)
		if err != nil {
			return err
		}
		image.PullRef = clearString(image.Name)
		refLookup[image.Name] = image.PullRef
	}

	for _, image := range dockerImages {
		if image.PullRef != "" {
			continue
		}
		if ref, ok := refLookup[image.Name]; ok {
			image.PullRef = ref
			continue
		}
		rc := registryCredentials.FindCredentialForImage(image.Name)
		_, err := downloadImageViaSkopeo(commands.ExecWithMessage, folderPath, image.Name, rc)
		if err != nil {
			return err
		}
		image.PullRef = clearString(image.Name)
		refLookup[image.Name] = image.PullRef
	}
	return nil
}

func downloadImageViaSkopeo(cmdExecutor cmdExecutor, folderPath, imageName string, rc *models.RegistryCredential) (string, error) {
	logrus.Infof("Downloading missing image %s", imageName)
	dockerURL := "docker://" + imageName
	archiveName := "docker-archive:" + folderPath + clearString(imageName)
	args := []string{"copy"}

	if rc != nil {
		if rc.Username == "<token>" {
			// --src-registry-token string
			args = append(args, "--src-registry-token")
			args = append(args, rc.Password)
		} else {
			// --src-creds USERNAME[:PASSWORD]
			args = append(args, "--src-creds")
			creds := rc.Username
			if rc.Password != "" {
				creds += ":" + rc.Password
			}
			args = append(args, creds)
		}
		logrus.Infof("using credentials: %v", *rc)
	}

	if os.Getenv("SKOPEO_ARGS") != "" {
		args = append(args, strings.Split(os.Getenv("SKOPEO_ARGS"), ",")...)
	}

	args = append(args, dockerURL, archiveName)
	return cmdExecutor(exec.Command("skopeo", args...), "pulling "+imageName)
}

// mergeImages - at this point, all images are downloaded at folderPath
func mergeImages(folderPath string, manifestImages []trivymodels.Image) ([]trivymodels.Image, error) {
	// Untar images, read manifest.json/RepoTags, match tags to YAML
	logrus.Infof("Extracting details for all images")
	allImages := []trivymodels.Image{}
	err := walkImages(folderPath, func(filename string, sha string, repoTags []string) {
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
			logrus.Warningf("Could not find repo tags for %s", filename)
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
		} else {
			repoAndTag := repoTags[0]
			repo := strings.Split(repoAndTag, ":")[0]
			image.ID = fmt.Sprintf("%s@%s", repo, sha)
			image.Name = repoAndTag
			image.Owner.Name = repo // This name is used for the filename in the Insights UI
		}

		allImages = append(allImages, *image)
	})
	return allImages, err
}

// scanImagesWithTrivy scans the images and returns a Trivy report ready to send to Insights.
func scanImagesWithTrivy(images []trivymodels.Image, configurationObject models.Configuration) ([]byte, string, error) {
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
		logrus.Infof("Scanning %s from file %s", currentImage.Name, currentImage.PullRef)
		results, err := ScanImageFile(configurationObject.Images.FolderName+currentImage.PullRef, currentImage.PullRef, configurationObject.Options.TempFolder, "")
		if err != nil {
			return nil, "", err
		}
		reportByRef[currentImage.PullRef] = results
	}

	allReports := image.ConvertTrivyResultsToImageReport(images, reportByRef, false)
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
		bytes, err := io.ReadAll(tarReader)
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
			var ok bool
			configFileName, ok = imageDef.(map[string]interface{})["Config"].(string)
			if !ok {
				logrus.Warningf("Found manifest with no Config at %s", path)
				continue
			}
			repoTags, ok := imageDef.(map[string]interface{})["RepoTags"].([]interface{})
			if !ok {
				logrus.Warningf("Found manifest with no RepoTags at %s", path)
				continue
			}
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
		bytes, err := io.ReadAll(tarReader)
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

// ScanImageFile will scan a single file with Trivy and return the results.
func ScanImageFile(imagePath, imageID, tempDir, extraFlags string) (*trivymodels.TrivyResults, error) {
	reportFile := tempDir + "/trivy-report-" + imageID + ".json"
	cmd := exec.Command("trivy", "-d", "image", "--skip-update", "-f", "json", "-o", reportFile, "--input", imagePath)
	if extraFlags != "" {
		cmd = exec.Command("trivy", "-d", "image", "--skip-update", extraFlags, "-f", "json", "-o", reportFile, "--input", imagePath)
	}
	err := util.RunCommand(cmd, "scanning "+imageID)
	if err != nil {
		logrus.Errorf("Error scanning %s at %s: %v", imageID, imagePath, err)
		return nil, err
	}
	defer func() {
		os.Remove(reportFile)
	}()

	report := trivymodels.TrivyResults{}
	data, err := os.ReadFile(reportFile)
	if err != nil {
		logrus.Errorf("Error reading report %s: %s", imageID, err)
		return nil, err
	}
	err = json.Unmarshal(data, &report)
	if err != nil {
		logrus.Errorf("Error decoding report %s: %s", imageID, err)
		return nil, err
	}

	return &report, nil
}

var imageFilenameRegex = regexp.MustCompile(`[^a-zA-Z0-9 ]+`)

// clearString - removes non-alphanumeric characters
func clearString(str string) string {
	return imageFilenameRegex.ReplaceAllString(str, "")
}
