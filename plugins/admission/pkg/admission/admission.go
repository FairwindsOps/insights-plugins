package admission

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/thoas/go-funk"

	"github.com/fairwindsops/insights-plugins/admission/pkg/models"
)

var organization string
var hostname string
var cluster string

func init() {
	organization = os.Getenv("FAIRWINDS_ORGANIZATION")
	hostname = os.Getenv("FAIRWINDS_HOSTNAME")
	cluster = os.Getenv("FAIRWINDS_CLUSTER")
}

// SendResults sends the results to Insights
func SendResults(reports []models.ReportInfo, token string) (passed bool, warnings []string, errors []string, err error) {
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

	url := fmt.Sprintf("%s/v0/organizations/%s/clusters/%s/data/admission/submit", hostname, organization, cluster)
	req, err := http.NewRequest("POST", url, &b)
	if err != nil {
		logrus.Warn("Unable to create Request")
		return
	}

	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	for _, report := range reports {
		req.Header.Set("X-Fairwinds-Report-Version-"+report.Report, report.Version)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logrus.Warn("Unable to Post results to Insights")
		return
	}
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("invalid status code: %d", resp.StatusCode)
		return
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logrus.Warn("Unable to read results")
		return
	}
	var resultMap map[string]interface{}
	err = json.Unmarshal(body, &resultMap)
	if err != nil {
		return
	}
	passed = resultMap["Success"].(bool)
	actionItems := resultMap["ActionItems"]
	if actionItems != nil {
		actionItemToString := func(ai interface{}) string {
			aiMap := ai.(map[string]interface{})
			return fmt.Sprintf("%s", aiMap["Title"].(string))
		}
		warnings = funk.Map(funk.Filter(actionItems.([]interface{}), func(ai interface{}) bool {
			return !ai.(map[string]interface{})["Failure"].(bool)
		}), actionItemToString).([]string)

		errors = funk.Map(funk.Filter(actionItems.([]interface{}), func(ai interface{}) bool {
			return ai.(map[string]interface{})["Failure"].(bool)
		}), actionItemToString).([]string)
	}

	logrus.Infof("Completed request %t", passed)
	return
}
