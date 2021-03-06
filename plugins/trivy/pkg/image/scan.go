package image

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"regexp"

	"github.com/fairwindsops/insights-plugins/trivy/pkg/models"
	"github.com/fairwindsops/insights-plugins/trivy/pkg/util"
	"github.com/sirupsen/logrus"
)

// TempDir is the directory to use for temporary storage.
const TempDir = "/output/tmp"
const retryCount = 3

var nonWordRegexp = regexp.MustCompile("\\W+")

// GetLastReport returns the last report for Trivy from Fairwinds Insights
func GetLastReport() models.MinimizedReport {
	url := os.Getenv("FAIRWINDS_INSIGHTS_HOST") + "/v0/organizations/" + os.Getenv("FAIRWINDS_ORG") + "/clusters/" + os.Getenv("FAIRWINDS_CLUSTER") + "/data/trivy/latest.json"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("FAIRWINDS_TOKEN"))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return models.MinimizedReport{Images: make([]models.ImageDetailsWithRefs, 0), Vulnerabilities: map[string]models.VulnerabilityDetails{}}
	}
	if resp.StatusCode != 200 {
		panic(fmt.Sprintf("Bad Status code on get last report: %d", resp.StatusCode))
	}
	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	var jsonResp models.MinimizedReport
	err = json.Unmarshal(responseBody, &jsonResp)
	if err != nil {
		panic(err)
	}

	return jsonResp

}

// ScanImages will download the set of images given and scan them with Trivy.
func ScanImages(images []models.Image, maxConcurrentScans int) []models.ImageReport {
	logrus.Infof("Scanning %d images", len(images))
	reportByRef := map[string][]models.VulnerabilityList{}
	for _, image := range images {
		reportByRef[image.PullRef] = nil
	}

	semaphore := make(chan bool, maxConcurrentScans)
	for pullRef := range reportByRef {
		semaphore <- true
		go func(pullRef string) {
			defer func() { <-semaphore }()
			for i := 0; i < retryCount; i++ { // Retry logic
				var err error
				reportByRef[pullRef], err = downloadAndScanPullRef(pullRef)
				if err == nil || err.Error() == util.UnknownOSMessage {
					break
				}
			}
		}(pullRef)
	}
	for i := 0; i < cap(semaphore); i++ {
		semaphore <- true
	}
	return ConvertTrivyResultsToImageReport(images, reportByRef)
}

// ConvertTrivyResultsToImageReport maps results from Trivy with metadata about the image scanned.
func ConvertTrivyResultsToImageReport(images []models.Image, reportByRef map[string][]models.VulnerabilityList) []models.ImageReport {
	allReports := make([]models.ImageReport, len(images))
	for idx, image := range images {
		allReports[idx] = models.ImageReport{
			Name:           image.Name,
			ID:             image.ID,
			PullRef:        image.PullRef,
			OwnerKind:      image.Owner.Kind,
			OwnerName:      image.Owner.Name,
			OwnerContainer: &images[idx].Owner.Container,
			Namespace:      image.Owner.Namespace,
			Report:         reportByRef[image.PullRef],
		}

	}
	return allReports
}

// ScanImageFile will scan a single file with Trivy and return the results.
func ScanImageFile(imagePath string, imageID string, tempDir string) ([]models.VulnerabilityList, error) {
	reportFile := tempDir + "/trivy-report-" + imageID + ".json"
	err := util.RunCommand(exec.Command("trivy", "--skip-update", "-d", "-f", "json", "-o", reportFile, "--input", imagePath), "scanning "+imageID)
	if err != nil {
		logrus.Errorf("Error scanning %s at %s: %v", imageID, imagePath, err)
		return nil, err
	}
	defer func() {
		os.Remove(reportFile)
	}()

	report := []models.VulnerabilityList{}
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

	return report, nil
}

func downloadAndScanPullRef(pullRef string) ([]models.VulnerabilityList, error) {
	imageID := nonWordRegexp.ReplaceAllString(pullRef, "_")

	imageDir := TempDir
	imageMessage := fmt.Sprintf("image %s", pullRef)

	err := util.RunCommand(exec.Command("skopeo", "copy", "docker://"+pullRef, "docker-archive:"+imageDir+imageID), "pulling "+imageMessage)
	defer func() {
		logrus.Info("removing " + imageID)
		os.Remove(imageDir + imageID)
	}()
	if err != nil {
		return nil, err
	}
	return ScanImageFile(imageDir+imageID, imageID, TempDir)
}
