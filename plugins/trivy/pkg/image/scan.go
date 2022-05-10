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
			defer func() { <-semaphore }()
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
	return ConvertTrivyResultsToImageReport(images, reportByRef, ignoreErrors)
}

// ConvertTrivyResultsToImageReport maps results from Trivy with metadata about the image scanned.
func ConvertTrivyResultsToImageReport(images []models.Image, reportResultByRef map[string]*models.TrivyResults, ignoreErrors bool) []models.ImageReport {
	allReports := []models.ImageReport{}
	for _, i := range images {
		image := i
		id := fmt.Sprintf("%s@%s", image.Name, GetShaFromID(image.ID))
		if t, ok := reportResultByRef[image.PullRef]; !ok || t == nil {
			if !ignoreErrors {
				allReports = append(allReports, models.ImageReport{
					Name:               image.Name,
					ID:                 id,
					PullRef:            image.PullRef,
					OwnerKind:          image.Owner.Kind,
					OwnerName:          image.Owner.Name,
					OwnerContainer:     &image.Owner.Container,
					Namespace:          image.Owner.Namespace,
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
			OwnerKind:          image.Owner.Kind,
			OwnerName:          image.Owner.Name,
			OwnerContainer:     &image.Owner.Container,
			Namespace:          image.Owner.Namespace,
			Reports:            trivyResult.Results,
			RecommendationOnly: image.RecommendationOnly,
		})
	}
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
	cmd := exec.Command("trivy", "-d", "image", "--skip-update", "-f", "json", "-o", reportFile, pullRef)
	if extraFlags != "" {
		cmd = exec.Command("trivy", "-d", "image", "--skip-update", extraFlags, "-f", "json", "-o", reportFile, pullRef)
	}
	err := util.RunCommand(cmd, "scanning "+pullRef)
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

	return &report, nil
}

func GetShaFromID(id string) string {
	if len(strings.Split(id, "@")) > 1 {
		return strings.Split(id, "@")[1]
	}
	return id
}
