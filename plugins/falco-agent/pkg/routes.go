package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/fairwindsops/controller-utils/pkg/controller"
	"github.com/fairwindsops/insights-plugins/falco-agent/pkg/data"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
)

const outputfolder = "/output"

func inputDataHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	payload, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logrus.Errorf("Error reading body: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	outputFile := fmt.Sprintf("%s/%s.json", outputfolder, strconv.FormatInt(time.Now().Unix(), 10))
	err = ioutil.WriteFile(outputFile, []byte(payload), 0644)
	if err != nil {
		logrus.Errorf("Error writting to file: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write([]byte(`{"status": "ok"}`))
}

func outputDataHandler(w http.ResponseWriter, r *http.Request, ctx context.Context, dynamicClient dynamic.Interface, restMapper meta.RESTMapper) {
	w.Header().Set("Content-Type", "application/json")
	payload, err := data.Aggregate24hrsData(outputfolder)
	if err != nil {
		logrus.Errorf("Error while aggregating data: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	requestArray := make([]data.FalcoOutput, 0, len(payload))

	workloads, err := controller.GetAllTopControllers(ctx, dynamicClient, restMapper, "")
	if err != nil {
		logrus.Errorf("Error while getting all TopControllers: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	workloadMap := make(map[string]controller.Workload)
	for _, workload := range workloads {
		for _, pod := range workload.Pods {
			workloadMap[fmt.Sprintf("%s/%s", pod.GetNamespace(), pod.GetName())] = workload
		}
	}
	for _, val := range payload {
		namespace := val.OutputFields["k8s.ns.name"].(string)
		podName := val.OutputFields["k8s.pod.name"].(string)
		workload, ok := workloadMap[fmt.Sprintf("%s/%s", namespace, podName)]
		val.ControllerNamespace = namespace
		val.PodName = podName
		if !ok {
			val.ControllerName, val.ControllerKind = data.GetController(workloads, podName, namespace)
		} else {
			val.ControllerName = workload.TopController.GetName()
			val.ControllerKind = workload.TopController.GetKind()
		}
		requestArray = append(requestArray, val)
	}
	output := data.OutputFormat{
		Output: requestArray,
	}
	data, err := json.Marshal(output)
	if err != nil {
		logrus.Errorf("Error on json.Marshal(payload): %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, err = w.Write([]byte(data))
	if err != nil {
		logrus.Errorf("Error while sending data: %v", err)
	}
}
