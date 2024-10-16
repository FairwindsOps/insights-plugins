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
	"slices"
	"strings"

	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/image"
	trivymodels "github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/util"
	"github.com/sirupsen/logrus"

	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/commands"
	"github.com/fairwindsops/insights-plugins/plugins/ci/pkg/models"
	ciutil "github.com/fairwindsops/insights-plugins/plugins/ci/pkg/util"
	"github.com/hashicorp/go-multierror"
)

func (ci *CIScan) GetTrivyReport(dockerImages []trivymodels.DockerImage, manifestImages []trivymodels.Image) (report *models.ReportInfo, errs error) {
	allErrs := new(multierror.Error)
	dockerImages, manifestImages, err := updatePullRef(ci.config.Images.FolderName, dockerImages, manifestImages)
	if err != nil {
		return nil, err
	}

	filenameToImageName, dockerImages, manifestImages, err := downloadMissingImages(ci.config.Images.FolderName, downloadImageViaSkopeo, dockerImages, manifestImages, ci.config.Options.RegistryCredentials)
	if err != nil {
		allErrs = multierror.Append(allErrs, err)
	}

	allImages, err := mergeImages(ci.config.Images.FolderName, dockerImages, manifestImages, filenameToImageName)
	if err != nil {
		logrus.Debugf("error returned while merging images: %v", err)
		multierror.Append(allErrs, err)
	}

	trivyResults, trivyVersion, err := scanImagesWithTrivy(allImages, *ci.config)
	if err != nil {
		return nil, multierror.Append(allErrs, err, models.ScanErrorsReportResult{
			ErrorMessage: err.Error(),
			ErrorContext: "running trivy",
			ResourceName: "unknown",
		})
	}
	if trivyResults != nil {
		filename := "trivy.json"
		err = os.WriteFile(filepath.Join(ci.config.Options.TempFolder, filename), trivyResults, 0644)
		if err != nil {
			return nil, multierror.Append(allErrs, err)
		}
		return &models.ReportInfo{
			Report:   "trivy",
			Filename: filename,
			Version:  trivyVersion,
		}, allErrs
	}
	return nil, allErrs.ErrorOrNil()
}

// filename -> postgres_15_1_bullseye
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
func updatePullRef(folderPath string, dockerImages []trivymodels.DockerImage, manifestImages []trivymodels.Image) ([]trivymodels.DockerImage, []trivymodels.Image, error) {
	logrus.Infof("Looking through images in %s", folderPath)
	err := walkImages(folderPath, func(filename string, sha string, repoTags []string) {
		for i := range manifestImages {
			image := &manifestImages[i]
			if slices.Contains(repoTags, image.Name) {
				// already downloaded
				logrus.Infof("image (manifest) %s already downloaded", image.Name)
				image.PullRef = filename
			}
		}

		for i := range dockerImages {
			image := &dockerImages[i]
			if slices.Contains(repoTags, image.Name) {
				// already downloaded
				logrus.Infof("image (docker) %s already downloaded", image.Name)
				image.PullRef = filename
			}
		}
	})
	if err != nil {
		return nil, nil, err
	}
	return dockerImages, manifestImages, nil
}

func downloadMissingImages(folderPath string, imageDownloaderFunc ImageDownloaderFunc, dockerImages []trivymodels.DockerImage, manifestImages []trivymodels.Image, registryCredentials models.RegistryCredentials) (map[string]string, []trivymodels.DockerImage, []trivymodels.Image, error) {
	allErrs := new(multierror.Error)
	refLookup := map[string]string{} // postgres:15.1-bullseye -> postgres_15_1_bullseye
	// Download missing images
	for i := range manifestImages {
		image := &manifestImages[i]
		if image.PullRef != "" {
			continue
		}
		if ref, ok := refLookup[image.Name]; ok {
			image.PullRef = ref
			continue
		}
		rc := registryCredentials.FindCredentialForImage(image.Name)
		output, err := imageDownloaderFunc(commands.ExecWithMessage, folderPath, image.Name, rc)
		if err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("%v: %s", err, output))
		} else {
			image.PullRef = clearString(image.Name)
			refLookup[image.Name] = image.PullRef
		}
	}

	for i := range dockerImages {
		image := &dockerImages[i]
		if image.PullRef != "" {
			continue
		}
		if ref, ok := refLookup[image.Name]; ok {
			image.PullRef = ref
			continue
		}
		rc := registryCredentials.FindCredentialForImage(image.Name)
		output, err := imageDownloaderFunc(commands.ExecWithMessage, folderPath, image.Name, rc)
		if err != nil {
			allErrs = multierror.Append(allErrs, fmt.Errorf("%v: %s", err, output))
		} else {
			image.PullRef = clearString(image.Name)
			refLookup[image.Name] = image.PullRef
		}
	}
	if len(allErrs.Errors) > 0 {
		return ciutil.ReverseMap(refLookup), dockerImages, manifestImages, models.ScanErrorsReportResult{
			ErrorMessage: allErrs.Error(), // keep multiple errors combined into a single error / action item
			ErrorContext: "downloading missing images to be scanned by trivy",
			Kind:         "InternalOperation",
			Remediation:  "Please inspect the action item description for signs of why downloading images may have failed.",
			ResourceName: "DownloadMissingImages",
			Filename:     filepath.Clean(folderPath), // clean() removes potential trailing slash
		}
	}
	return ciutil.ReverseMap(refLookup), dockerImages, manifestImages, nil // postgres_15_1_bullseye -> postgres:15.1-bullseye
}

// ImageDownloaderFunc - downloads an image and returns the output and error
type ImageDownloaderFunc = func(cmdExecutor cmdExecutor, folderPath, imageName string, rc *models.RegistryCredential) (string, error)

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
	output, err := cmdExecutor(exec.Command("skopeo", args...), "pulling "+imageName)
	if err != nil {
		archiveFileName := folderPath + clearString(imageName)
		logrus.Infof("cleaning up file %q left behind by the failed image-copy for %s", archiveFileName, imageName)
		removeErr := os.Remove(archiveFileName)
		if removeErr != nil {
			logrus.Errorf("unable to remove file %q, this will likely cause the trivy scan to fail for this empty file: %v", archiveFileName, removeErr)
		}
		return output, err
	}
	return output, nil
}

// mergeImages - at this point, all images are downloaded at folderPath
func mergeImages(folderPath string, dockerImages []trivymodels.DockerImage, manifestImages []trivymodels.Image, filenameToImageName map[string]string) ([]trivymodels.Image, error) {
	// Untar images, read manifest.json/RepoTags, match tags to YAML
	logrus.Infof("Extracting details for all images")
	allImages := []trivymodels.Image{}
	err := walkImages(folderPath, func(filename string, sha string, repoTags []string) {
		logrus.Infof("Getting details for image file %s with SHA %s and repoTags %v", filename, sha, repoTags)
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
			var name, ownerName string
			if n, ok := filenameToImageName[filename]; ok {
				name = n
				ownerName = determineOwnerName(n, dockerImages)
			}
			image = &trivymodels.Image{
				Name:    name,
				PullRef: filename,
				Owners: []trivymodels.Resource{
					{
						Name: ownerName,
						Kind: "Image",
					},
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
			image.Owners[0].Name = repo // This name is used for the filename in the Insights UI
		}

		allImages = append(allImages, *image)
	})
	return allImages, err
}

func determineOwnerName(n string, dockerImages []trivymodels.DockerImage) string {
	// if found on dockerImages, the owner name is `fairwinds-insights.yaml``
	for _, di := range dockerImages {
		if di.Name == n {
			return configFileName
		}
	}

	parts := strings.Split(n, ":")
	if len(parts) > 1 {
		return parts[0]
	}

	return ""
}

// scanImagesWithTrivy scans the images and returns a Trivy report ready to send to Insights.
// Multiple errors may be returned.
func scanImagesWithTrivy(images []trivymodels.Image, configurationObject models.Configuration) (report []byte, reportVersion string, errs error) {
	allErrs := new(multierror.Error)
	trivyVersion, err := commands.Exec("trivy", "--version")
	if err != nil {
		return nil, "", fmt.Errorf("unable to get trivy version: %v", err)
	}
	trivyVersion = strings.Split(strings.Split(trivyVersion, "\n")[0], " ")[1]
	output, err := commands.ExecWithMessage(exec.Command("trivy", "image", "--download-db-only"), "downloading trivy database")
	if err != nil {
		return nil, "", fmt.Errorf("unable to download trivy database, %v: %s", err, output)
	}
	reportByRef := map[string]*trivymodels.TrivyResults{}
	errorsByRef := map[string]*multierror.Error{}
	for _, currentImage := range images {
		_, ok := reportByRef[currentImage.PullRef]
		if ok {
			continue
		}
		logrus.Infof("Scanning %s from file %s", currentImage.Name, currentImage.PullRef)
		results, err := ScanImageFile(configurationObject.Images.FolderName+currentImage.PullRef, currentImage.PullRef, configurationObject.Options.TempFolder, "")
		if err != nil {
			logrus.Errorf("error scanning %s from file %s: %v", currentImage.Name, currentImage.PullRef, err)
			scanError := models.ScanErrorsReportResult{
				ErrorMessage: err.Error(),
				ErrorContext: "running trivy to scan image",
				Kind:         "Image",
				ResourceName: currentImage.Name,
				Filename:     currentImage.PullRef,
			}
			errorsByRef[currentImage.PullRef] = multierror.Append(errorsByRef[currentImage.PullRef], scanError)
			allErrs = multierror.Append(allErrs, scanError)
		}
		reportByRef[currentImage.PullRef] = results
	}

	allReports := image.ConvertTrivyResultsToImageReport(images, reportByRef, errorsByRef)
	// Collate results
	results := image.Minimize(allReports, trivymodels.MinimizedReport{Images: make([]trivymodels.ImageDetailsWithRefs, 0), Vulnerabilities: map[string]trivymodels.VulnerabilityDetails{}})
	trivyResults, err := json.Marshal(results)
	if err != nil {
		return nil, "", allErrs.ErrorOrNil()
	}

	return trivyResults, trivyVersion, allErrs.ErrorOrNil()
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
	_, err := util.RunCommand(cmd, "scanning "+imageID)
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
	return imageFilenameRegex.ReplaceAllString(str, "_")
}
