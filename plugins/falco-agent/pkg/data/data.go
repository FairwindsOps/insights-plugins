package data

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"

	"github.com/fairwindsops/controller-utils/pkg/log"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
			logrus.Info(filename)
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

// GetPodByPodName returns pod from the namespace and name provided.
func GetPodByPodName(ctx context.Context, dynamicClient dynamic.Interface, restMapper meta.RESTMapper, namespace, podname string) (*unstructured.Unstructured, error) {
	fqKind := schema.FromAPIVersionAndKind("v1", "Pod")
	mapping, err := restMapper.RESTMapping(fqKind.GroupKind(), fqKind.Version)
	if err != nil {
		log.GetLogger().Error(err, "Error retrieving mapping", "v1", "Pod")
		return nil, err
	}
	pod, err := dynamicClient.Resource(mapping.Resource).Namespace(namespace).Get(ctx, podname, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return pod, nil
}
