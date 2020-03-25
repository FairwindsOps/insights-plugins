package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

const tmpDir = "/output/tmp"
const outputFile = tmpDir + "/final-report.json"
const unknownOSMessage = "Unknown OS"
const retryCount = 3

var nonWordRegexp = regexp.MustCompile("\\W+")
var maxConcurrentScans = 5
var numberToScan = 10

type ImageReport struct {
	Name      string
	ID        string
	PullRef   string
	OwnerKind string
	OwnerName string
	Namespace string
	Report    []VulnerabilityList
}

type VulnerabilityList struct {
	Target          string
	Vulnerabilities []Vulnerability
}

type Vulnerability struct {
	Title            string
	Description      string
	InstalledVersion string
	FixedVersion     string
	PkgName          string
	Severity         string
	VulnerabilityID  string
	References       []string
}

func main() {

	concurrencyStr := os.Getenv("MAX_CONCURRENT_SCANS")
	if concurrencyStr != "" {
		var err error
		maxConcurrentScans, err = strconv.Atoi(concurrencyStr)
		if err != nil {
			panic(err)
		}
	}

	numberToScanStr := os.Getenv("MAX_SCANS")
	if numberToScanStr != "" {
		var err error
		numberToScan, err = strconv.Atoi(numberToScanStr)
		if err != nil {
			panic(err)
		}
	}

	err := runCommand(exec.Command("trivy", "--download-db-only"), "downloading trivy database")
	if err != nil {
		panic(err)
	}
	checkEnvironmentVariables()
	lastReport := getLastReport()
	images, err := GetImages()
	if err != nil {
		panic(err)
	}

	imagesToScan := make([]Image, 0)
	for _, image := range images {
		found := false

		for _, report := range lastReport.Images {
			if report.Name == image.Name && report.ID == image.ID {
				found = true
				break
			}
		}
		if !found {
			imagesToScan = append(imagesToScan, image)
		}
	}
	imagesToKeep := make([]ImageDetailsWithRefs, 0)
	sort.Slice(lastReport.Images, func(a, b int) bool {
		return lastReport.Images[a].LastScan == nil || lastReport.Images[b].LastScan != nil && lastReport.Images[a].LastScan.Before(*lastReport.Images[b].LastScan)
	})
	for _, report := range lastReport.Images {
		keep := false
		for _, image := range images {
			if report.Name == image.Name && report.ID == image.ID {
				if len(imagesToScan) < numberToScan {
					imagesToScan = append(imagesToScan, image)
					break
				}
				keep = true
				break
			}
		}
		if keep {
			imagesToKeep = append(imagesToKeep, report)
		}
	}
	lastReport.Images = imagesToKeep
	if len(imagesToScan) > numberToScan {
		imagesToScan = imagesToScan[:numberToScan]
	}
	allReports := scanImages(imagesToScan)
	finalReport := minimize(allReports, lastReport)

	data, err := json.Marshal(finalReport)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(outputFile, data, 0644)
	if err != nil {
		panic(err)
	}
}

func checkEnvironmentVariables() {
	if os.Getenv("FAIRWINDS_INSIGHTS_HOST") == "" || os.Getenv("FAIRWINDS_ORG") == "" || os.Getenv("FAIRWINDS_CLUSTER") == "" || os.Getenv("FAIRWINDS_TOKEN") == "" {
		panic("Proper environment variables not set.")
	}

}

func getLastReport() MinimizedReport {
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
		return MinimizedReport{Images: make([]ImageDetailsWithRefs, 0), Vulnerabilities: map[string]VulnerabilityDetails{}}
	}
	if resp.StatusCode != 200 {
		panic(fmt.Sprintf("Bad Status code on get last report: %d", resp.StatusCode))
	}
	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	var jsonResp MinimizedReport
	err = json.Unmarshal(responseBody, &jsonResp)
	if err != nil {
		panic(err)
	}

	return jsonResp

}

func scanImages(images []Image) []ImageReport {
	logrus.Infof("Scanning %d images", len(images))
	reportByRef := map[string][]VulnerabilityList{}
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
				reportByRef[pullRef], err = runTrivy(pullRef)
				if err == nil || err.Error() == unknownOSMessage {
					break
				}
			}
		}(pullRef)
	}
	for i := 0; i < cap(semaphore); i++ {
		semaphore <- true
	}

	allReports := make([]ImageReport, len(images))
	for idx, image := range images {
		allReports[idx] = ImageReport{
			Name:      image.Name,
			ID:        image.ID,
			PullRef:   image.PullRef,
			OwnerKind: image.Owner.Kind,
			OwnerName: image.Owner.Name,
			Namespace: image.Owner.Namespace,
			Report:    reportByRef[image.PullRef],
		}
	}
	return allReports
}

func runTrivy(pullRef string) ([]VulnerabilityList, error) {
	imageID := nonWordRegexp.ReplaceAllString(pullRef, "_")
	reportFile := tmpDir + "/trivy-report-" + imageID + ".json"
	imageDir := tmpDir
	imageMessage := fmt.Sprintf("image %s", pullRef)

	err := runCommand(exec.Command("skopeo", "copy", "docker://"+pullRef, "docker-archive:"+imageDir+imageID), "pulling "+imageMessage)
	defer func() {
		logrus.Info("removing " + imageID)
		os.Remove(imageDir + imageID)
		os.Remove(reportFile)
	}()
	if err != nil {
		return nil, err
	}
	err = runCommand(exec.Command("trivy", "--skip-update", "-d", "-f", "json", "-o", reportFile, "--input", imageDir+imageID), "scanning "+imageMessage)
	if err != nil {
		return nil, err
	}

	report := []VulnerabilityList{}
	data, err := ioutil.ReadFile(reportFile)
	if err != nil {
		logrus.Errorf("Error reading report %s: %s", pullRef, err)
		return nil, err
	}
	err = json.Unmarshal(data, &report)
	if err != nil {
		logrus.Errorf("Error decoding report %s: %s", pullRef, err)
		return nil, err
	}

	return report, nil
}

func runCommand(cmd *exec.Cmd, message string) error {
	logrus.Info(message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logrus.Errorf("Error %s: %s\n%s", message, err, string(output))
		if strings.Contains(output, unknownOSMessage) {
			return errors.New(unknownOSMessage)
		}
	}
	return err
}
