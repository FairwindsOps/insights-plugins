package data

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/fairwindsops/controller-utils/pkg/controller"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
)

func isLessThan24hrs(t time.Time) bool {
	return time.Now().Sub(t) < 24*time.Hour
}

func deleteOlderFile(filePath string) (err error) {
	err = os.Remove(filePath)
	if err != nil {
		return

	}
	return
}

func readDataFromFile(fileName string) (payload FalcoOutput, err error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		return
	}
	err = json.Unmarshal(data, &payload)
	if err != nil {
		return
	}
	return
}

// Aggregate24hrsData return aggregated report for the past 24 hours
func Aggregate24hrsData(dir string) (aggregatedData []FalcoOutput, err error) {
	tmpfiles, err := ioutil.ReadDir(dir)
	if err != nil {
		return
	}

	for _, file := range tmpfiles {
		if file.Mode().IsRegular() {
			filename := filepath.Join(dir, file.Name())
			if isLessThan24hrs(file.ModTime()) {
				var output FalcoOutput
				output, err = readDataFromFile(filename)
				if err != nil {
					return
				}
				aggregatedData = append(aggregatedData, output)
			} else {
				err = deleteOlderFile(filename)
				if err != nil {
					return
				}
			}
		}
	}
	return
}

// GetController returns the controller name and kind
func GetController(workloads []controller.Workload, podName, namespace, repository string) (name, kind, container string) {
	name = podName
	kind = "Pod"
	for _, workload := range workloads {
		if workload.TopController.GetNamespace() != namespace {
			continue
		}
		for _, pod := range workload.Pods {
			if pod.GetName() == podName {
				// Exact match for a pod, go ahead and return
				name = workload.TopController.GetName()
				kind = workload.TopController.GetKind()

				var pd corev1.Pod
				err := runtime.DefaultUnstructuredConverter.
					FromUnstructured(pod.UnstructuredContent(), &pd)
				if err != nil {
					logrus.Errorf("Error Converting Pod GetController: %v", err)
					return
				}
				for _, ctn := range pd.Spec.Containers {
					if strings.HasPrefix(ctn.Image, repository) {
						container = ctn.Name
					}
				}
				return
			}
		}
		// 5 digit alphanumeric (pod) or strictly numeric segments (cronjob -> job, statefulset). or 9 digit alphanumberic (deployment -> rs)
		matched, err := regexp.Match(fmt.Sprintf("^%s-([a-z0-9]{5}|[a-z0-9]{9}|[0-9]*)(-[a-z0-9]{5})?$", workload.TopController.GetName()), []byte(podName))
		if err != nil {
			logrus.Error(err)
			return
		}
		if matched {
			// Weak match for a pod. Don't return yet in case there's a better match.
			name = workload.TopController.GetName()
			kind = workload.TopController.GetKind()
		}
	}
	return
}
