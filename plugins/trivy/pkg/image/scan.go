package image

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/util"
	"github.com/sirupsen/logrus"
)

// TempDir is the directory to use for temporary storage.
const TempDir = "/output/tmp"
const retryCount = 3

var nonWordRegexp = regexp.MustCompile("\\W+")

var registryPassword = os.Getenv("REGISTRY_PASSWORD")
var registryUser = os.Getenv("REGISTRY_USER")
var registryCertDir = os.Getenv("REGISTRY_CERT_DIR")

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

// GetLastReport returns the last report for Trivy from Fairwinds Insights
func GetLastReport(host, org, cluster, token string) (*models.MinimizedReport, error) {
	url := fmt.Sprintf("%s/v0/organizations/%s/clusters/%s/data/trivy/latest.json", host, org, cluster)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return &models.MinimizedReport{Images: make([]models.ImageDetailsWithRefs, 0), Vulnerabilities: map[string]models.VulnerabilityDetails{}}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Bad Status code on get last report: %d", resp.StatusCode)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return unmarshalBody(body)
}

func unmarshalBody(body []byte) (*models.MinimizedReport, error) {
	var report models.MinimizedReport
	err := json.Unmarshal(body, &report)
	if err != nil {
		return nil, err
	}
	fixOwners(&report)
	return &report, nil
}

// fixOwners adapt older owners fields to the new ones
func fixOwners(report *models.MinimizedReport) {
	for i := range report.Images {
		img := &report.Images[i]
		if hasDeprecatedOwner(*img) {
			var container string
			if img.OwnerContainer != nil {
				container = *img.OwnerContainer
			}
			img.Owners = []models.Resource{
				{
					Name:      img.OwnerName,
					Kind:      img.OwnerKind,
					Namespace: img.Namespace,
					Container: container,
				},
			}
		}
	}
}

func hasDeprecatedOwner(img models.ImageDetailsWithRefs) bool {
	return len(img.OwnerName) != 0 || len(img.OwnerKind) != 0 || len(img.Namespace) != 0
}

// ScanImages will download the set of images given and scan them with Trivy.
func ScanImages(images []models.Image, maxConcurrentScans int, extraFlags string, ignoreErrors bool) []models.ImageReport {
	logrus.Infof("Scanning %d images", len(images))
	reportByRef := map[string]*models.TrivyResults{}
	for _, image := range images {
		reportByRef[image.PullRef] = nil
	}
	semaphore := make(chan bool, maxConcurrentScans)
	for pullRef := range reportByRef {
		semaphore <- true
		go func(pullRef string) {
			defer func() {
				logrus.Infof("Finished scanning %s", pullRef)
				<-semaphore
			}()
			for i := 0; i < retryCount; i++ { // Retry logic
				var err error
				r, err := ScanImage(extraFlags, pullRef)
				reportByRef[pullRef] = r
				if err == nil || err.Error() == util.UnknownOSMessage {
					break
				}
			}
		}(pullRef)
	}
	for i := 0; i < cap(semaphore); i++ {
		semaphore <- true
	}
	logrus.Infof("Finished scanning all images")
	return ConvertTrivyResultsToImageReport(images, reportByRef, ignoreErrors)
}

// ConvertTrivyResultsToImageReport maps results from Trivy with metadata about the image scanned.
func ConvertTrivyResultsToImageReport(images []models.Image, reportResultByRef map[string]*models.TrivyResults, ignoreErrors bool) []models.ImageReport {
	logrus.Infof("Converting results to image report")
	allReports := []models.ImageReport{}
	for _, i := range images {
		image := i
		id := fmt.Sprintf("%s@%s", image.Name, image.GetSha())
		if t, ok := reportResultByRef[image.PullRef]; !ok || t == nil {
			if !ignoreErrors {
				allReports = append(allReports, models.ImageReport{
					Name:               image.Name,
					ID:                 id,
					PullRef:            image.PullRef,
					Owners:             image.Owners,
					RecommendationOnly: image.RecommendationOnly,
				})
			}
			continue
		}
		trivyResult := reportResultByRef[image.PullRef]
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
		logrus.Errorf("Error while downloading image: %v", err)
		return nil, err
	}
	defer func() {
		logrus.Infof("removing image file %s", imageFile)
		os.Remove(imageFile)
	}()
	args = append(args, "--input", imageFile)
	cmd := exec.Command("trivy", args...)
	err = util.RunCommand(cmd, "scanning "+pullRef)

	if err != nil {
		logrus.Errorf("Error scanning %s: %v", pullRef, err)
		return nil, err
	}
	defer func() {
		os.Remove(reportFile)
	}()

	report := models.TrivyResults{}
	data, err := ioutil.ReadFile(reportFile)
	if err != nil {
		logrus.Errorf("Error reading report %s: %s", imageID, err)
		return nil, err
	}
	err = json.Unmarshal(data, &report)
	if err != nil {
		logrus.Errorf("Error decoding report %s: %s", imageID, err)
		return nil, err
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
