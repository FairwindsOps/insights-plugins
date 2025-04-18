package admission

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/samber/lo"
	"github.com/sirupsen/logrus"

	admissionversion "github.com/fairwindsops/insights-plugins/plugins/admission"
	"github.com/fairwindsops/insights-plugins/plugins/admission/pkg/models"
)

// sendResults sends the results to Insights
func sendResults(iConfig models.InsightsConfig, reports []models.ReportInfo) (passed bool, warnings []string, errors []string, err error) {
	var b bytes.Buffer

	w := multipart.NewWriter(&b)

	for _, report := range reports {
		var fw io.Writer
		fw, err = w.CreateFormFile(report.Report, report.Report+".json")
		if err != nil {
			logrus.Warnf("Unable to create form for %s", report.Report)
			return false, nil, nil, err
		}
		_, err = fw.Write(report.Contents)
		logrus.Debugf("Adding report %s %s", report.Report, string(report.Contents))
		if err != nil {
			logrus.Warnf("Unable to write contents for %s", report.Report)
			return
		}
	}
	w.Close()

	url := fmt.Sprintf("%s/v0/organizations/%s/clusters/%s/data/admission/submit", iConfig.Hostname, iConfig.Organization, iConfig.Cluster)
	req, err := http.NewRequest("POST", url, &b)
	if err != nil {
		logrus.Warnf("Unable to create Request")
		return
	}

	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+iConfig.Token)
	req.Header.Set("X-Fairwinds-Admission-Version", admissionversion.String())
	for _, report := range reports {
		req.Header.Set("X-Fairwinds-Report-Version-"+report.Report, report.Version)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logrus.Warnf("Unable to Post results to Insights")
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logrus.Warnf("Unable to read results")
		return
	}
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("invalid status code: %d - %s", resp.StatusCode, string(body))
		return
	}
	var resultMap map[string]interface{}
	err = json.Unmarshal(body, &resultMap)
	if err != nil {
		logrus.Warnf("Unable to unmarshal results")
		return
	}
	passed = resultMap["Success"].(bool)
	actionItems := resultMap["ActionItems"]
	if actionItems != nil {
		actionItemToString := func(ai interface{}, _ int) string {
			aiMap := ai.(map[string]interface{})
			return fmt.Sprintf("%s", aiMap["Title"].(string))
		}
		warnings = lo.Map(lo.Filter(actionItems.([]interface{}), func(ai interface{}, _ int) bool {
			return !ai.(map[string]interface{})["Failure"].(bool)
		}), actionItemToString)

		errors = lo.Map(lo.Filter(actionItems.([]interface{}), func(ai interface{}, _ int) bool {
			return ai.(map[string]interface{})["Failure"].(bool)
		}), actionItemToString)
	}
	if message, ok := resultMap["Message"]; ok {
		if str, stringOK := message.(string); stringOK && len(message.(string)) > 0 {
			warnings = append(warnings, str)
		}
	}
	logrus.Infof("Completed request %t", passed)
	return
}
