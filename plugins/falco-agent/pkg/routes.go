package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fairwindsops/controller-utils/pkg/controller"
	"github.com/fairwindsops/insights-plugins/plugins/falco-agent/pkg/data"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
)

const outputfolder = "/output"

func inputDataHandler(w http.ResponseWriter, r *http.Request, ctx context.Context, dynamicClient dynamic.Interface, restMapper meta.RESTMapper) {
	w.Header().Set("Content-Type", "application/json")
	body, err := io.ReadAll(r.Body)
	if err != nil {
		logrus.Errorf("Error reading body: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var falcoOutput data.FalcoOutput
	logrus.Info(falcoOutput)

	err = json.Unmarshal(body, &falcoOutput)
	if err != nil {
		logrus.Errorf("Error unmarshalling body: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var namespace, podName, repository string

	if falcoOutput.OutputFields["k8s.ns.name"] != nil {
		namespace = falcoOutput.OutputFields["k8s.ns.name"].(string)
	}

	if falcoOutput.OutputFields["k8s.pod.name"] != nil {
		podName = falcoOutput.OutputFields["k8s.pod.name"].(string)
	}
	if falcoOutput.OutputFields["container.image.repository"] != nil {
		repository = falcoOutput.OutputFields["container.image.repository"].(string)
	}
	if namespace == "" || podName == "" {
		logrus.Error("Failed to get namespace or podName")
		http.Error(w, "Failed to get namespace or podName", http.StatusInternalServerError)
		return
	}

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
	falcoOutput.ControllerName = controller.GetName()
	falcoOutput.ControllerKind = controller.GetKind()
	falcoOutput.ControllerNamespace = namespace
	falcoOutput.PodName = podName

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
			falcoOutput.Container = ctn.Name
		}
	}

	payload, err := json.Marshal(falcoOutput)
	if err != nil {
		logrus.Errorf("Error Converting Pod: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	outputFile := fmt.Sprintf("%s/%s.json", outputfolder, strconv.FormatInt(time.Now().Unix(), 10))
	err = os.WriteFile(outputFile, []byte(payload), 0644)
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
