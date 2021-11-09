package report

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/golang/glog"
	core "k8s.io/api/core/v1"
	kube_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Should we import from github.com/fairwindsops/insights/pkg/reports instead
// of defining the same types here?
// RightSizerReportItem shows the right-sizer-item property
type RightSizerReportItem struct {
	Kind              string             `json:"kind"`
	ResourceName      string             `json:"resourceName"`
	ResourceNamespace string             `json:"resourceNamespace"`
	ResourceContainer string             `json:"resourceContainer"`
	StartingMemory    *resource.Quantity `json:"startingMemory"`
	EndingMemory      *resource.Quantity `json:"endingMemory"`
}

// RightSizerReportProperties shows the right-sizer-item property
type RightSizerReportProperties struct {
	Items []RightSizerReportItem `json:"items"`
}

// RightSizerReport is a report from right-sizer-report
type RightSizerReport struct {
	// Version string // I believe this is not needed, supplied by the Insights uploader
	Report RightSizerReportProperties
}

type RightSizerReportBuilder struct {
	Report         *RightSizerReport
	itemsLock      *sync.RWMutex
	HTTPServer     *http.Server
	outputFileName string
}

func (i RightSizerReportItem) String() string {
	return fmt.Sprintf("%s %s/%s:%s", i.Kind, i.ResourceNamespace, i.ResourceName, i.ResourceContainer)
}

// NewRightSizerReportBuilder returns a pointer to a new initialized
// RightSizerReportBuilder type.
func NewRightSizerReportBuilder() *RightSizerReportBuilder {
	b := &RightSizerReportBuilder{
		Report:    &RightSizerReport{},
		itemsLock: &sync.RWMutex{},
		HTTPServer: &http.Server{
			ReadTimeout:  2500 * time.Millisecond, // time to read request headers and   optionally body
			WriteTimeout: 10 * time.Second,
		},
		outputFileName: "output.json",
	}
	return b
}

func (b *RightSizerReportBuilder) GetReport() *RightSizerReport {
	b.itemsLock.RLock()
	defer b.itemsLock.RUnlock()
	return b.Report
}

// alreadyHave accepts a RightSizerReportItem and returns true if that item
// already exists in the RightSizerReportProperties.
// ONly kind, namespace, name, and container name are matched.
func (b *RightSizerReportBuilder) AlreadyHave(newItem RightSizerReportItem) bool {
	b.itemsLock.RLock()
	defer b.itemsLock.RUnlock()
	for _, item := range b.Report.Report.Items {
		if item.Kind == newItem.Kind && item.ResourceNamespace == newItem.ResourceNamespace && item.ResourceName == newItem.ResourceName && item.ResourceContainer == newItem.ResourceContainer {
			return true
		}
	}
	return false
}

func (b *RightSizerReportBuilder) AddItem(newItem RightSizerReportItem) {
	b.itemsLock.Lock()
	defer b.itemsLock.Unlock()
	b.Report.Report.Items = append(b.Report.Report.Items, newItem)
}

func (b *RightSizerReportBuilder) GetReportJSON() ([]byte, error) {
	b.itemsLock.RLock()
	defer b.itemsLock.RUnlock()
	data, err := json.MarshalIndent(b.Report, "", "  ")
	if err != nil {
		glog.Errorf("cannot marshal report: %v", err)
		return nil, err
	}
	return data, nil
}

func (b *RightSizerReportBuilder) ReportHandler(w http.ResponseWriter, r *http.Request) {
	data, err := b.GetReportJSON()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, err = w.Write(data)
	if err != nil {
		glog.Errorf("error sending report data: %v", err)
	}
}

func (b *RightSizerReportBuilder) WriteOutputFile() error {
	data, err := b.GetReportJSON()
	if err != nil {
		return err
	}
	err = os.WriteFile(b.outputFileName, data, 0600)
	if err != nil {
		return err
	}
	return nil
}

func (b RightSizerReportBuilder) RunServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/report", b.ReportHandler)
	b.HTTPServer.Handler = mux
	b.HTTPServer.Addr = "localhost:8080" // should become a parameter to this function
	go func() { glog.Fatal(b.HTTPServer.ListenAndServe()) }()

}

func (b *RightSizerReportBuilder) WriteConfigMap(kubeClient kubernetes.Interface, namespaceName, configMapName string) error {
	updateTime := time.Now().String()
	var configMap *core.ConfigMap
	reportBytes, err := b.GetReportJSON()
	if err != nil {
		return err
	}
	reportJSON := string(reportBytes)
	configMaps := kubeClient.CoreV1().ConfigMaps(namespaceName)
	configMap, err = configMaps.Get(context.TODO(), configMapName, meta.GetOptions{})
	if err == nil {
		// Update the report and time-stamp annotation.
		configMap.Data["report"] = reportJSON
		if configMap.ObjectMeta.Annotations == nil {
			configMap.ObjectMeta.Annotations = make(map[string]string)
		}
		configMap.ObjectMeta.Annotations["last-updated"] = updateTime
		_, err = configMaps.Update(context.TODO(), configMap, meta.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("unable to update ConfigMap %s/%s to save report: %w", namespaceName, configMapName, err)
		}
	}

	if kube_errors.IsNotFound(err) {
		// No ConfigMap in-cluster, create one.
		configMap = &core.ConfigMap{
			ObjectMeta: meta.ObjectMeta{
				Namespace: namespaceName,
				Name:      configMapName,
				Annotations: map[string]string{
					"last-updated": updateTime,
				},
			},
			Data: map[string]string{
				"report": reportJSON,
			},
		}
		_, err = configMaps.Create(context.TODO(), configMap, meta.CreateOptions{})
		if err != nil {
			return fmt.Errorf("unable to create ConfigMap %s/%s to save report: %w", namespaceName, configMapName, err)
		}
	}

	if err != nil && !kube_errors.IsNotFound(err) {
		// An unexpected error
		return fmt.Errorf("unable to get ConfigMap %s/%s to save report: %w", namespaceName, configMapName, err)
	}
	return nil
}
