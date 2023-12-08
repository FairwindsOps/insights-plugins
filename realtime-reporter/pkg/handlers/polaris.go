package handlers

import (
	"encoding/json"
	"fmt"
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

func PolarisHandler(resourceType string) cache.ResourceEventHandlerFuncs {

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
		logrus.Infof("%s", eventJson)
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
			logrus.Infof("%s", eventJson)
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

		logrus.Infof("%s", eventJson)
	}
	return handler
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
