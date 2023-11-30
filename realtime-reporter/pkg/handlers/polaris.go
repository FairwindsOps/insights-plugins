package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/admission/pkg/models"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"

	evt "github.com/FairwindsOps/insights-plugins/realtime-reporter/pkg/event"
	"github.com/FairwindsOps/insights-plugins/realtime-reporter/pkg/polaris"
)

const maxTries = 3

func PolarisHandler(resourceType string) cache.ResourceEventHandlerFuncs {

	var handler cache.ResourceEventHandlerFuncs
	handler.AddFunc = func(obj interface{}) {
		bytes, err := json.Marshal(obj)
		if err != nil {
			logrus.Errorf("Unable to marshal object: %v", err)
		}

		u := obj.(*unstructured.Unstructured)

		report, err := polaris.GetPolarisReport(bytes)
		if err != nil {
			logrus.Errorf("Unable to retrieve polaris report: %v", err)
		}

		reportMap := polarisReportInfoToMap(report)
		e := evt.NewEvent(u.GetObjectKind().GroupVersionKind().Kind, u.GetNamespace(), u.GetName(), reportMap)
		eventJson, err := json.Marshal(e)
		if err != nil {
			logrus.Errorf("Unable to marshal event: %v", err)
		}
		err = uploadToInsights(eventJson)
		if err != nil {
			logrus.Errorf("unable to upload to Insights: %v", err)
		}
	}
	handler.UpdateFunc = func(old, new interface{}) {
		bytes, err := json.Marshal(new)
		if err != nil {
			logrus.Errorf("Unable to marshal object: %v", err)
		}

		u := new.(*unstructured.Unstructured)

		report, err := polaris.GetPolarisReport(bytes)
		if err != nil {
			logrus.Errorf("Unable to retrieve polaris report: %v", err)
		}

		reportMap := polarisReportInfoToMap(report)
		e := evt.NewEvent(u.GetObjectKind().GroupVersionKind().Kind, u.GetNamespace(), u.GetName(), reportMap)
		eventJson, err := json.Marshal(e)
		if err != nil {
			logrus.Errorf("Unable to marshal event: %v", err)
		}
		err = uploadToInsights(eventJson)
		if err != nil {
			logrus.Errorf("unable to upload to Insights: %v", err)
		}
	}
	handler.DeleteFunc = func(obj interface{}) {
		// an event with empty data currently indicates the resource has been removed
		u := obj.(*unstructured.Unstructured)

		e := evt.NewEvent(u.GetObjectKind().GroupVersionKind().Kind, u.GetNamespace(), u.GetName(), nil)
		eventJson, err := json.Marshal(e)
		if err != nil {
			logrus.Errorf("Unable to marshal event: %v", err)
		}
		err = uploadToInsights(eventJson)
		if err != nil {
			logrus.Errorf("unable to upload to Insights: %v", err)
		}
	}
	return handler
}

func uploadToInsights(payload []byte) error {
	organization, cluster, reportType, token := "acme-co", "vvezani-test", "polaris", "jt8IDB8IDa8kONPTX2j_UW09RHpuEpbGUbeKciH2jIUFb9DyaoEGuWxplfwmH_51"

	var sendError bool
	var tries int

	for {
		apiURL := fmt.Sprintf("http://localhost:3001/v0/organizations/%s/clusters/%s/data/%s/incremental", organization, cluster, reportType)
		if sendError {
			apiURL = fmt.Sprintf("http://localhost:3001/v0/organizations/%s/clusters/%s/data/%s/incremental/failure", organization, cluster, reportType)
		}

		req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payload))
		if err != nil {
			return fmt.Errorf("Error creating HTTP request: %v", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token) // Add your authentication token if needed

		// Create an HTTP client and send the request
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("Error making HTTP request: %v", err)
		}
		defer resp.Body.Close()

		// Check the response
		if resp.StatusCode == http.StatusOK {
			print(string(payload))
			break
		} else {
			fmt.Println("Failed to upload event - status code:", resp.StatusCode)

			if sendError {
				return fmt.Errorf("failed to upload event - status code: %d - tried %d times", resp.StatusCode, tries-1) // failed to upload error
			}

			if tries >= maxTries {
				time.Sleep(100 * time.Millisecond)
				sendError = true
			}
			logrus.Warnf("failed to upload event - status code: %d - trying again...[%d/%d]", resp.StatusCode, tries, maxTries)
			tries++
		}
	}

	return nil
}

type PolarisReport struct {
	Report   string
	Version  string
	Contents []byte
}

func polarisReportInfoToMap(report models.ReportInfo) map[string]interface{} {
	m := PolarisReport{
		Report:   report.Report,
		Version:  report.Version,
		Contents: report.Contents,
	}

	var v map[string]any
	err := json.Unmarshal(m.Contents, &v)
	if err != nil {
		panic(err)
	}

	im := map[string]any{
		"Report":   m.Report,
		"Version":  m.Version,
		"Contents": m.Contents,
	}

	reportJson, err := json.Marshal(im)
	if err != nil {
		panic(err)
	}

	var reportMap map[string]interface{}
	err = json.Unmarshal(reportJson, &reportMap)
	if err != nil {
		panic(err)
	}

	return reportMap
}
