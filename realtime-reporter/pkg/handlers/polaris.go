package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/admission/pkg/models"
	"github.com/go-test/deep"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"

	evt "github.com/FairwindsOps/insights-plugins/realtime-reporter/pkg/event"
	"github.com/FairwindsOps/insights-plugins/realtime-reporter/pkg/polaris"
)

const maxTries = 3
const host = "http://localhost:3001"
const organization, cluster = "acme-co", "vvezani"

func PolarisHandler(token string, resourceType string) cache.ResourceEventHandlerFuncs {

	var handler cache.ResourceEventHandlerFuncs
	handler.AddFunc = func(obj interface{}) {
		timestamp := getTimestampUnixNanos()
		logrus.WithField("resourceType", resourceType).Info("add event")

		bytes, err := json.Marshal(obj)
		if err != nil {
			logrus.Errorf("Unable to marshal object: %v", err)
		}

		u := obj.(*unstructured.Unstructured)
		ownerReferences := u.GetOwnerReferences()

		// if owner references exist, and the object controller is configured to be watched, we can discard this event since the
		// controller will be used to process the report
		if len(ownerReferences) > 0 {
			for _, controller := range ownerReferences {
				if lo.Contains(viper.GetStringSlice("resources"), getOwnerReferenceGVKStringPluralized(controller)) {
					return
				}
			}
		}

		report, err := polaris.GetPolarisReport(bytes)
		if err != nil {
			logrus.Errorf("Unable to retrieve polaris report: %v", err)
		}

		reportMap := polarisReportInfoToMap(report)
		e := evt.NewEvent(timestamp, u.GetObjectKind().GroupVersionKind().Kind, u.GetNamespace(), u.GetName(), reportMap)
		eventJson, err := json.Marshal(e)
		if err != nil {
			logrus.Errorf("Unable to marshal event: %v", err)
		}
		err = uploadToInsights(token, timestamp, eventJson)
		if err != nil {
			logrus.Errorf("unable to upload to Insights: %v", err)
		}
	}
	handler.UpdateFunc = func(old, new interface{}) {
		timestamp := getTimestampUnixNanos()

		oldObj := old.(*unstructured.Unstructured)
		newObj := new.(*unstructured.Unstructured)
		oldObjSpec, _, _ := unstructured.NestedMap(oldObj.Object, "spec")
		newObjSpec, _, _ := unstructured.NestedMap(newObj.Object, "spec")
		// TODO:
		// something like this, but we need to ignore certain managed fields
		// oldObjMeta, _, _ := unstructured.NestedMap(oldObj.Object, "metadata")
		// newObjMeta, _, _ := unstructured.NestedMap(newObj.Object, "metadata")

		if !equality.Semantic.DeepEqual(oldObjSpec, newObjSpec) {
			specDiff := deep.Equal(oldObjSpec, newObjSpec)
			logrus.WithField("resourceType", resourceType).WithField("specDiff", specDiff).Info("update event")

			bytes, err := json.Marshal(new)
			if err != nil {
				logrus.Errorf("Unable to marshal object: %v", err)
			}

			report, err := polaris.GetPolarisReport(bytes)
			if err != nil {
				logrus.Errorf("Unable to retrieve polaris report: %v", err)
			}

			reportMap := polarisReportInfoToMap(report)

			e := evt.NewEvent(timestamp, newObj.GetObjectKind().GroupVersionKind().Kind, newObj.GetNamespace(), newObj.GetName(), reportMap)
			eventJson, err := json.Marshal(e)
			if err != nil {
				logrus.Errorf("Unable to marshal event: %v", err)
			}
			err = uploadToInsights(token, timestamp, eventJson)
			if err != nil {
				logrus.Errorf("unable to upload to Insights: %v", err)
			}
		}
	}
	handler.DeleteFunc = func(obj interface{}) {
		timestamp := getTimestampUnixNanos()
		logrus.WithField("resourceType", resourceType).Info("delete event")

		// an event with empty data currently indicates the resource has been removed
		u := obj.(*unstructured.Unstructured)

		e := evt.NewEvent(timestamp, u.GetObjectKind().GroupVersionKind().Kind, u.GetNamespace(), u.GetName(), nil)
		eventJson, err := json.Marshal(e)
		if err != nil {
			logrus.Errorf("Unable to marshal event: %v", err)
		}
		err = uploadToInsights(token, timestamp, eventJson)
		if err != nil {
			logrus.Errorf("unable to upload to Insights: %v", err)
		}
	}
	return handler
}

func uploadToInsights(token string, timestamp int64, payload []byte) error {
	reportType := "polaris"

	var sendError bool
	var tries int

	for {
		apiURL := fmt.Sprintf("%s/v0/organizations/%s/clusters/%s/data/%s/incremental", host, organization, cluster, reportType)
		if sendError {
			apiURL = fmt.Sprintf("%s/v0/organizations/%s/clusters/%s/data/%s/incremental/failure", host, organization, cluster, reportType)
		}

		req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payload))
		if err != nil {
			return fmt.Errorf("Error creating HTTP request: %v", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Fairwinds-Event-Timestamp", fmt.Sprintf("%d", timestamp))
		req.Header.Set("Authorization", "Bearer "+token) // Add your authentication token if needed

		// Create an HTTP client and send the request
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("Error making HTTP request: %v", err)
		}
		defer resp.Body.Close()

		// Check the response
		if resp.StatusCode == http.StatusOK {
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

func getTimestampUnixNanos() int64 {
	return time.Now().UTC().UnixNano()
}

func getOwnerReferenceGVKStringPluralized(ownerReference metav1.OwnerReference) string {
	return strings.ToLower(fmt.Sprintf("%v/%vs", ownerReference.APIVersion, ownerReference.Kind))
}
