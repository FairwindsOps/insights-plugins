package data

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func isLessThan24hrs(t time.Time) bool {
	return time.Since(t) < 24*time.Hour
}

func deleteOlderFile(fsWrapper afero.Fs, filePath string) (err error) {
	err = fsWrapper.Remove(filePath)
	if err != nil {
		return

	}
	return
}

func readDataFromFile(fsWrapper afero.Fs, fileName string) (payload FalcoOutput, err error) {
	data, err := afero.ReadFile(fsWrapper, fileName)
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
func Aggregate24hrsData(fsWrapper afero.Fs, dir string) (aggregatedData []FalcoOutput, err error) {
	tmpfiles, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var fileStat fs.FileInfo

	for _, file := range tmpfiles {
		if file.Type().Perm().IsRegular() {
			filename := filepath.Join(dir, file.Name())
			fileStat, err = fsWrapper.Stat(filename)
			if err != nil {
				return
			}
			if isLessThan24hrs(fileStat.ModTime()) {
				var output FalcoOutput
				output, err = readDataFromFile(fsWrapper, filename)
				// skip malformed files and continue
				if err != nil {
					logrus.Warnf("Error reading file: %v", err)
					continue
				}
				aggregatedData = append(aggregatedData, output)
			} else {
				err = deleteOlderFile(fsWrapper, filename)
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
		logrus.Error(err, "Error retrieving mapping", "v1", "Pod")
		return nil, err
	}
	pod, err := dynamicClient.Resource(mapping.Resource).Namespace(namespace).Get(ctx, podname, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return pod, nil
}
