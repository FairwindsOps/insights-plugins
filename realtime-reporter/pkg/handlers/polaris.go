package handlers

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fairwindsops/insights-plugins/plugins/admission/pkg/models"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"

	fwClient "github.com/FairwindsOps/insights-plugins/realtime-reporter/pkg/client"
	evt "github.com/FairwindsOps/insights-plugins/realtime-reporter/pkg/event"
	"github.com/FairwindsOps/insights-plugins/realtime-reporter/pkg/polaris"
)

func PolarisHandler(resourceType string) cache.ResourceEventHandlerFuncs {

	var handler cache.ResourceEventHandlerFuncs
	handler.AddFunc = func(obj interface{}) {
		timestamp := getTimestampUnixNanos()
		logrus.WithField("resourceType", resourceType).Debug("add event")

		bytes, err := json.Marshal(obj)
		if err != nil {
			panic(err)
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
			panic(err)
		}

		reportMap, err := polarisReportInfoToMap(report)
		if err != nil {
			panic(err)
		}

		e := evt.NewEvent(timestamp, "add", u.GetObjectKind().GroupVersionKind().Kind, u.GetNamespace(), u.GetName(), reportMap)
		eventJson, err := json.Marshal(e)
		if err != nil {
			panic(err)
		}

		err = fwClient.UploadToInsights(timestamp, "polaris", eventJson)
		if err != nil {
			panic(err)
		}
	}
	handler.UpdateFunc = func(old, new interface{}) {
		timestamp := getTimestampUnixNanos()
		logrus.WithField("resourceType", resourceType).Debug("update event")

		oldObj := old.(*unstructured.Unstructured)
		newObj := new.(*unstructured.Unstructured)

		if oldObj.GetGeneration() < newObj.GetGeneration() {
			bytes, err := json.Marshal(new)
			if err != nil {
				panic(err)
			}

			report, err := polaris.GetPolarisReport(bytes)
			if err != nil {
				panic(err)
			}

			reportMap, err := polarisReportInfoToMap(report)
			if err != nil {
				panic(err)
			}

			e := evt.NewEvent(timestamp, "update", newObj.GetObjectKind().GroupVersionKind().Kind, newObj.GetNamespace(), newObj.GetName(), reportMap)
			eventJson, err := json.Marshal(e)
			if err != nil {
				panic(err)
			}

			err = fwClient.UploadToInsights(timestamp, "polaris", eventJson)
			if err != nil {
				panic(err)
			}
		}
	}
	handler.DeleteFunc = func(obj interface{}) {
		timestamp := getTimestampUnixNanos()
		logrus.WithField("resourceType", resourceType).Debug("delete event")

		// an event with empty data currently indicates the resource has been removed
		u := obj.(*unstructured.Unstructured)

		e := evt.NewEvent(timestamp, "delete", u.GetObjectKind().GroupVersionKind().Kind, u.GetNamespace(), u.GetName(), nil)
		eventJson, err := json.Marshal(e)
		if err != nil {
			panic(err)
		}

		err = fwClient.UploadToInsights(timestamp, "polaris", eventJson)
		if err != nil {
			panic(err)
		}
	}
	return handler
}

type PolarisReport struct {
	Report   string
	Version  string
	Contents []byte
}

func polarisReportInfoToMap(report models.ReportInfo) (map[string]interface{}, error) {
	m := PolarisReport{
		Report:   report.Report,
		Version:  report.Version,
		Contents: report.Contents,
	}

	var v map[string]any
	err := json.Unmarshal(m.Contents, &v)
	if err != nil {
		return nil, err
	}

	im := map[string]any{
		"Report":   m.Report,
		"Version":  m.Version,
		"Contents": m.Contents,
	}

	reportJson, err := json.Marshal(im)
	if err != nil {
		return nil, err
	}

	var reportMap map[string]interface{}
	err = json.Unmarshal(reportJson, &reportMap)

	return reportMap, err
}

func getTimestampUnixNanos() int64 {
	return time.Now().UTC().UnixNano()
}

func getOwnerReferenceGVKStringPluralized(ownerReference metav1.OwnerReference) string {
	return strings.ToLower(fmt.Sprintf("%v/%vs", ownerReference.APIVersion, ownerReference.Kind))
}
