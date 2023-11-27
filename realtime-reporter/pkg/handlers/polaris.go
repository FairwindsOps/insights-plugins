package handlers

import (
	"encoding/json"

	"github.com/fairwindsops/insights-plugins/plugins/admission/pkg/models"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"

	evt "github.com/FairwindsOps/insights-plugins/realtime-reporter/pkg/event"
	"github.com/FairwindsOps/insights-plugins/realtime-reporter/pkg/polaris"
)

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
		e := evt.NewEvent("polaris", u.GetNamespace(), u.GetName(), reportMap)
		eventJson, _ := json.Marshal(e)
		logrus.Infof("%s", eventJson)
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
		e := evt.NewEvent("polaris", u.GetNamespace(), u.GetName(), reportMap)
		eventJson, _ := json.Marshal(e)
		logrus.Infof("%s", eventJson)
	}
	handler.DeleteFunc = func(obj interface{}) {
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
		e := evt.NewEvent("polaris", u.GetNamespace(), u.GetName(), reportMap)
		eventJson, _ := json.Marshal(e)
		logrus.Debugf("%s", eventJson)
	}
	return handler
}

func polarisReportInfoToMap(report models.ReportInfo) map[string]interface{} {
	reportJson, _ := json.Marshal(report)
	var reportMap map[string]interface{}
	err := json.Unmarshal(reportJson, &reportMap)
	if err != nil {
		logrus.Errorf("Unable to unmarshal polaris report: %v", err)
		return nil
	}
	return reportMap
}
