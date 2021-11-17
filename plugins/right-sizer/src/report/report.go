package report

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	core "k8s.io/api/core/v1"
	kube_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// RightSizerReportItem contains properties of a right-sizer item.
type RightSizerReportItem struct {
	Kind              string             `json:"kind"`
	ResourceName      string             `json:"resourceName"`
	ResourceNamespace string             `json:"resourceNamespace"`
	ResourceContainer string             `json:"resourceContainer"`
	StartingMemory    *resource.Quantity `json:"startingMemory"`
	EndingMemory      *resource.Quantity `json:"endingMemory"`
	NumOOMs           int64              `json:"numOOMs"`
	FirstOOM          time.Time          `json:"firstOOM"`
	LastOOM           time.Time          `json:"LastOOM"`
}

// RightSizerReportProperties holds multiple right-sizer-item properties.
type RightSizerReportProperties struct {
	Items []RightSizerReportItem `json:"items"`
}

// RightSizerReportBuilder holds internal resources required to create a
// RightSizerReport, and provides methods to manipulate the report.
type RightSizerReportBuilder struct {
	Report     *RightSizerReportProperties
	itemsLock  *sync.RWMutex
	HTTPServer *http.Server // Allows retrieving the report
}

// String represents unique fields of a report item.
func (i RightSizerReportItem) String() string {
	return fmt.Sprintf("%s %s/%s:%s", i.Kind, i.ResourceNamespace, i.ResourceName, i.ResourceContainer)
}

// NewRightSizerReportBuilder returns a pointer to a new initialized
// RightSizerReportBuilder type.
func NewRightSizerReportBuilder() *RightSizerReportBuilder {
	b := &RightSizerReportBuilder{
		Report:    &RightSizerReportProperties{},
		itemsLock: &sync.RWMutex{},
		HTTPServer: &http.Server{
			ReadTimeout:  2500 * time.Millisecond, // time to read request headers and   optionally body
			WriteTimeout: 10 * time.Second,
		},
	}
	return b
}

// AddOrUpdateItem accepts a new RightSizerReportItem and adds or updates it
// in the report. The NumOOMKills and LastOOM fields are automatically updated
// if the item already exists in the report.
func (b *RightSizerReportBuilder) AddOrUpdateItem(newItem RightSizerReportItem) {
	b.itemsLock.Lock()
	defer b.itemsLock.Unlock()
	for i, item := range b.Report.Items {
		if item.Kind == newItem.Kind && item.ResourceNamespace == newItem.ResourceNamespace && item.ResourceName == newItem.ResourceName && item.ResourceContainer == newItem.ResourceContainer {
			// UPdate the existing item.
			b.Report.Items[i].NumOOMs++
			b.Report.Items[i].LastOOM = time.Now()
			glog.V(1).Infof("updating OOMKill information for existing report item %s", item)
			return
		}
	}
	// This item is new to this report.
	newItem.NumOOMs = 1
	newItem.FirstOOM = time.Now()
	newItem.LastOOM = newItem.FirstOOM
	b.Report.Items = append(b.Report.Items, newItem)
	glog.V(1).Infof("adding new report item %s", newItem)
	return
}

// GetReportJSON( returns a RightSizerReport in JSON form.
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

// ReportHandler handles HTTP requests by serving the report as JSON.
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

// RunServer starts the report builder HTTP server and registers the report
// handler.
func (b RightSizerReportBuilder) RunServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/report", b.ReportHandler)
	b.HTTPServer.Handler = mux
	b.HTTPServer.Addr = "localhost:8080" // should become a parameter to this function
	go func() { glog.Fatal(b.HTTPServer.ListenAndServe()) }()
}

// WriteConfigMap writes the report to a Kubernetes ConfigMap resource. IF the
// ConfigMap does not exist, it will be created.
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
	if err != nil && !kube_errors.IsNotFound(err) {
		// This is an unexpected error.
		return fmt.Errorf("unable to get ConfigMap %s/%s to write report: %w", namespaceName, configMapName, err)
	}
	if err == nil {
		// Update existing ConfigMap with the report and time-stamp annotation.
		configMap.Data["report"] = reportJSON
		if configMap.ObjectMeta.Annotations == nil {
			configMap.ObjectMeta.Annotations = make(map[string]string)
		}
		configMap.ObjectMeta.Annotations["last-updated"] = updateTime
		_, err = configMaps.Update(context.TODO(), configMap, meta.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("unable to update ConfigMap %s/%s to write report: %w", namespaceName, configMapName, err)
		}
	}
	if kube_errors.IsNotFound(err) {
		// No ConfigMap found, create one.
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
			return fmt.Errorf("unable to create ConfigMap %s/%s to write report: %w", namespaceName, configMapName, err)
		}
	}
	return nil
}

// ReadConfigMap reads the report from a Kubernetes ConfigMap resource. IF
// the ConfigMap does not exist, the report remains unchanged.
// This is useful to populate the report with previous data on Kubernetes controller startup.
func (b *RightSizerReportBuilder) ReadConfigMap(kubeClient kubernetes.Interface, namespaceName, configMapName string) error {
	var configMap *core.ConfigMap
	var reportJSON string
	configMaps := kubeClient.CoreV1().ConfigMaps(namespaceName)
	configMap, err := configMaps.Get(context.TODO(), configMapName, meta.GetOptions{})
	if kube_errors.IsNotFound(err) {
		// No ConfigMap found; no data to populate the report.
		glog.V(1).Infof("ConfigMap %s/%s does not exist, the report will not be populated with existing state", namespaceName, configMapName)
		return nil
	}
	if err != nil && !kube_errors.IsNotFound(err) {
		// This is an unexpected error.
		return fmt.Errorf("unable to get ConfigMap %s/%s to read report state: %w", namespaceName, configMapName, err)
	}
	if err == nil {
		// Read the report from the ConfigMap
		var ok bool
		reportJSON, ok = configMap.Data["report"]
		if !ok {
			return fmt.Errorf("unable to read report from ConfigMap %s/%s as it does not contain a report key", namespaceName, configMapName)
		}
		glog.V(2).Infof("got report JSON from ConfigMap %s/%s: %q", namespaceName, configMapName, reportJSON)
		dec := json.NewDecoder(strings.NewReader(reportJSON))
		dec.DisallowUnknownFields() // Return an error if unmarshalling unexpected fields.
		err = dec.Decode(&b.Report)
		if err != nil {
			return fmt.Errorf("unable to unmarshal while reading report from ConfigMap %s/%s: %v", namespaceName, configMapName, err)
		}
		glog.V(2).Infof("Read report from ConfigMap %s/%s: %#v", namespaceName, configMapName, b.Report)
	}
	return nil
}
