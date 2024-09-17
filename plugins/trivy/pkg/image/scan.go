package image

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/util"
	"github.com/hashicorp/go-multierror"
	"github.com/sirupsen/logrus"
)

// TempDir is the directory to use for temporary storage.
const TempDir = "/output/tmp"
const retryCount = 3

var nonWordRegexp = regexp.MustCompile("\\W+")

var registryPassword = os.Getenv("REGISTRY_PASSWORD")
var registryUser = os.Getenv("REGISTRY_USER")
var registryCertDir = os.Getenv("REGISTRY_CERT_DIR")

type ImageScannerFunc = func(extraFlags, pullRef string) (*models.TrivyResults, error)

func init() {
	passwordFile := os.Getenv("REGISTRY_PASSWORD_FILE")
	if passwordFile != "" {
		logrus.Infof("Reading registry password from %s", passwordFile)
		content, err := os.ReadFile(passwordFile)
		if err != nil {
			panic(err)
		}
		registryPassword = string(content)
	}
}

// ScanImages will download the set of images given and scan them with Trivy.
func ScanImages(imgScanner ImageScannerFunc, images []models.Image, maxConcurrentScans int, extraFlags string) []models.ImageReport {
	logrus.Infof("Scanning %d images", len(images))
	reportByRef := map[string]*models.TrivyResults{}
	errorsByRef := map[string]*multierror.Error{}
	for _, image := range images {
		reportByRef[image.PullRef] = nil
		errorsByRef[image.PullRef] = nil
	}
	semaphore := make(chan bool, maxConcurrentScans)
	for pullRef := range reportByRef {
		semaphore <- true
		go func(pullRef string) {
			defer func() {
				<-semaphore
			}()
			for i := 0; i < retryCount; i++ { // Retry logic
				var err error
				r, err := imgScanner(extraFlags, pullRef)
				reportByRef[pullRef] = r
				if err == nil {
					logrus.Infof("successfully scanned %s", pullRef)
					break
				}
				errorsByRef[pullRef] = multierror.Append(errorsByRef[pullRef], err)
				if err.Error() == util.UnknownOSMessage {
					logrus.Errorf("known error scanning  %s: %v", pullRef, err)
					break
				}
				lastTry := i == retryCount-1
				if lastTry {
					logrus.Errorf("error scanning %s: %v [%d/%d]... giving up", pullRef, err, i+1, retryCount)
				} else {
					logrus.Errorf("error scanning %s [%d/%d]... retrying", pullRef, i+1, retryCount)
				}
			}
		}(pullRef)
	}
	for i := 0; i < cap(semaphore); i++ {
		semaphore <- true
	}
	logrus.Infof("Finished scanning all images")
	return ConvertTrivyResultsToImageReport(images, reportByRef, errorsByRef)
}

// ConvertTrivyResultsToImageReport maps results from Trivy with metadata about the image scanned.
func ConvertTrivyResultsToImageReport(images []models.Image, reportResultByRef map[string]*models.TrivyResults, trivyErrors map[string]*multierror.Error) []models.ImageReport {
	logrus.Infof("Converting results to image report")
	allReports := []models.ImageReport{}
	for _, i := range images {
		image := i
		id := fmt.Sprintf("%s@%s", image.Name, image.GetSha())
		trivyResult, found := reportResultByRef[image.PullRef]
		if !found || trivyResult == nil {
			if i.RecommendationOnly {
				continue // don't report on failed recommendation only images
			}
			allReports = append(allReports, models.ImageReport{
				ID:                 id,
				Name:               image.Name,
				PullRef:            image.PullRef,
				Owners:             image.Owners,
				RecommendationOnly: image.RecommendationOnly,
				Error:              extractLastError(image.PullRef, trivyErrors),
			})
			continue
		}
		if !strings.Contains(id, "sha256:") {
			id = fmt.Sprintf("%s@%s", image.Name, trivyResult.Metadata.ImageID)
			if len(trivyResult.Metadata.RepoDigests) > 0 {
				id = trivyResult.Metadata.RepoDigests[0]
			}
		}
		allReports = append(allReports, models.ImageReport{
			ID:                 id,
			Name:               image.Name,
			OSArch:             getOsArch(trivyResult.Metadata.ImageConfig),
			PullRef:            image.PullRef,
			Owners:             image.Owners,
			Reports:            trivyResult.Results,
			RecommendationOnly: image.RecommendationOnly,
		})
	}
	logrus.Infof("Done converting results to image report")
	return allReports
}

func extractLastError(pullRef string, trivyErrors map[string]*multierror.Error) string {
	if multiError, ok := trivyErrors[pullRef]; ok && multiError != nil {
		length := multiError.Len()
		if length > 0 {
			return multiError.Errors[length-1].Error()
		}
	}
	return ""
}

func getOsArch(imageCfg models.TrivyImageConfig) string {
	if imageCfg.OS == "" || imageCfg.Architecture == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s", imageCfg.OS, imageCfg.Architecture)
}

// ScanImage will scan a single image with Trivy and return the results.
func ScanImage(extraFlags, pullRef string) (*models.TrivyResults, error) {
	imageID := nonWordRegexp.ReplaceAllString(pullRef, "_")
	reportFile := TempDir + "/trivy-report-" + imageID + ".json"
	args := []string{"-d", "image", "--skip-update", "--security-checks", "vuln", "-f", "json", "-o", reportFile}
	if extraFlags != "" {
		args = append(args, extraFlags)
	}
	if os.Getenv("OFFLINE") != "" {
		args = append(args, "--offline-scan")
	}

	if refReplacements := os.Getenv("PULL_REF_REPLACEMENTS"); refReplacements != "" {
		replacements := strings.Split(refReplacements, ";")
		for _, replacement := range replacements {
			parts := strings.Split(replacement, ",")
			if len(parts) != 2 {
				logrus.Errorf("PULL_REF_REPLACEMENTS is badly formatted, can't interpret %s", replacement)
				continue
			}
			pullRef = strings.ReplaceAll(pullRef, parts[0], parts[1])
			logrus.Infof("Replaced %s with %s, pullRef is now %s", parts[0], parts[1], pullRef)
		}
	}

	logrus.Infof("Downloading image %s", pullRef)
	imageFile, err := downloadPullRef(pullRef)
	if err != nil {
		return nil, fmt.Errorf("error while downloading image: %w", err)
	}
	defer func() {
		logrus.Infof("removing image file %s", imageFile)
		os.Remove(imageFile)
	}()
	args = append(args, "--input", imageFile)
	cmd := exec.Command("trivy", args...)
	err = util.RunCommand(cmd, "scanning "+pullRef)

	if err != nil {
		return nil, fmt.Errorf("error scanning %s: %w", pullRef, err)
	}
	defer func() {
		os.Remove(reportFile)
	}()

	report := models.TrivyResults{}
	data, err := os.ReadFile(reportFile)
	if err != nil {
		return nil, fmt.Errorf("error reading report %s: %w", imageID, err)
	}
	err = json.Unmarshal(data, &report)
	if err != nil {
		return nil, fmt.Errorf("error decoding report %s: %w", imageID, err)
	}

	logrus.Infof("Successfully scanned %s", imageID)
	return &report, nil
}

func downloadPullRef(pullRef string) (string, error) {
	imageID := nonWordRegexp.ReplaceAllString(pullRef, "_")
	dest := TempDir + imageID
	imageMessage := fmt.Sprintf("image %s", pullRef)

	args := []string{"copy"}

	if os.Getenv("SKOPEO_ARGS") != "" {
		args = append(args, strings.Split(os.Getenv("SKOPEO_ARGS"), ",")...)
	}
	if os.Getenv("TRIVY_INSECURE") != "" {
		logrus.Warn("Skipping TLS verification for Skopeo")
		args = append(args, "--src-tls-verify=false")
		args = append(args, "--dest-tls-verify=false")
	}
	skipLog := false
	if registryUser != "" || registryPassword != "" {
		skipLog = true
		args = append(args, "--src-creds", registryUser+":"+registryPassword)
	}
	if registryCertDir != "" {
		args = append(args, "--src-cert-dir", registryCertDir)
	}

	// needed when running locally on mac
	// args = append(args, "--override-arch", "amd64")
	// args = append(args, "--override-os", "linux")

	args = append(args, "docker://"+pullRef, "docker-archive:"+dest)
	if !skipLog {
		logrus.Infof("Running command: skopeo %s", strings.Join(args, " "))
	}
	err := util.RunCommand(exec.Command("skopeo", args...), "pulling "+imageMessage)
	return dest, err
}
