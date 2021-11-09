package report

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/api/resource"
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
	Version string
	Report  RightSizerReportProperties
}

type RightSizerReportBuilder struct {
	Report     *RightSizerReport
	itemsLock  *sync.RWMutex
	HTTPServer *http.Server
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

func (b *RightSizerReportBuilder) ReportHandler(w http.ResponseWriter, r *http.Request) {
	data, err := json.Marshal(b.Report)
	if err != nil {
		glog.Errorf("cannot marshal report: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, err = w.Write([]byte(data))
	if err != nil {
		glog.Errorf("error sending report data: %v", err)
	}
}

func (b RightSizerReportBuilder) RunServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/report", b.ReportHandler)
	b.HTTPServer.Handler = mux
	b.HTTPServer.Addr = "localhost:8080" // should become a parameter to this function
	go func() { glog.Fatal(b.HTTPServer.ListenAndServe()) }()

}
