package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fairwindsops/controller-utils/pkg/controller"
	"github.com/fairwindsops/insights-plugins/falco-agent/pkg/data"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
)

const outputfolder = "/output"

func inputDataHandler(w http.ResponseWriter, r *http.Request, ctx context.Context, dynamicClient dynamic.Interface, restMapper meta.RESTMapper) {
	w.Header().Set("Content-Type", "application/json")
	payload, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logrus.Errorf("Error reading body: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var output data.FalcoOutput

	err = json.Unmarshal(payload, &output)
	if err != nil {
		logrus.Errorf("Error unmarshalling body: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	namespace := output.OutputFields["k8s.ns.name"].(string)
	podName := output.OutputFields["k8s.pod.name"].(string)
	repository := output.OutputFields["container.image.repository"].(string)
	pod, err := data.GetPodByPodName(ctx, dynamicClient, restMapper, namespace, podName)
	if err != nil {
		logrus.Errorf("Error retrieving pod using podname: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	controller, err := controller.GetTopController(ctx, dynamicClient, restMapper, *pod, nil)
	if err != nil {
		logrus.Errorf("Error retrieving Top Controller using podname: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	output.ControllerName = controller.GetName()
	output.ControllerKind = controller.GetKind()

	var pd corev1.Pod
	err = runtime.DefaultUnstructuredConverter.
		FromUnstructured(pod.UnstructuredContent(), &pd)
	if err != nil {
		logrus.Errorf("Error Converting Pod: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, ctn := range pd.Spec.Containers {
		if strings.HasPrefix(ctn.Image, repository) {
			output.Container = ctn.Name
		}
	}

	pyload, err := json.Marshal(output)
	if err != nil {
		logrus.Errorf("Error Converting Pod: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	outputFile := fmt.Sprintf("%s/%s.json", outputfolder, strconv.FormatInt(time.Now().Unix(), 10))
	err = ioutil.WriteFile(outputFile, []byte(pyload), 0644)
	if err != nil {
		logrus.Errorf("Error writting to file: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write([]byte(`{"status": "ok"}`))
}

func outputDataHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	payload, err := data.Aggregate24hrsData(outputfolder)
	if err != nil {
		logrus.Errorf("Error while aggregating data: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	output := data.OutputFormat{
		Output: payload,
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
