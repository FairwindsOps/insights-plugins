package report

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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
	LastOOM           time.Time          `json:"lastOOM"`
	ResourceVersion   string             `json:"ResourceVersion"`
}

// RightSizerReportProperties holds multiple right-sizer-items
type RightSizerReportProperties struct {
	Items []RightSizerReportItem `json:"items"`
}

// RightSizerReportBuilder holds internal resources required to create and
// manipulate a RightSizerReport
type RightSizerReportBuilder struct {
	Report                                *RightSizerReportProperties
	itemsLock                             *sync.RWMutex
	tooOldAge                             time.Duration        // When an item should be removed based on its age
	configMapNamespaceName, configMapName string               // ConfigMap name to persist report state
	HTTPServer                            *http.Server         // Allows retrieving the current report
	kubeClient                            kubernetes.Interface // Used to read/write state ConfigMap
}

// String represents unique fields to differentiate a report item.
func (i RightSizerReportItem) String() string {
	return fmt.Sprintf("%s %s/%s:%s", i.Kind, i.ResourceNamespace, i.ResourceName, i.ResourceContainer)
}

// NewRightSizerReportBuilder returns a pointer to a new initialized
// RightSizerReportBuilder type.
// TODO: populate defaults (state ConfigMap, too-old-age) from CLI flags.
func NewRightSizerReportBuilder(kubeClient kubernetes.Interface) *RightSizerReportBuilder {
	b := &RightSizerReportBuilder{
		Report:     &RightSizerReportProperties{},
		itemsLock:  &sync.RWMutex{},
		kubeClient: kubeClient,
		tooOldAge:  8.64e+13, // 24 HRs
		// tooOldAge: 6e+10, // 1 minute
		// tooOldAge:              3e+11, // 5 minutes
		configMapNamespaceName: "insights-agent",
		configMapName:          "right-sizer-controller-state",
		HTTPServer: &http.Server{
			ReadTimeout:  2500 * time.Millisecond, // time to read request headers and   optionally body
			WriteTimeout: 10 * time.Second,
		},
	}
	return b
}

// AddOrUpdateItem accepts a RightSizerReportItem and adds or updates it
// in the report.
// If the item already exists, `NumOOMs` and `LastOOM` fields are updated.
func (b *RightSizerReportBuilder) AddOrUpdateItem(newItem RightSizerReportItem) {
	b.itemsLock.Lock()
	defer b.itemsLock.Unlock()
	for i, item := range b.Report.Items {
		if item.Kind == newItem.Kind && item.ResourceNamespace == newItem.ResourceNamespace && item.ResourceName == newItem.ResourceName && item.ResourceContainer == newItem.ResourceContainer {
			// UPdate the existing item.
			b.Report.Items[i].NumOOMs++
			b.Report.Items[i].ResourceVersion = newItem.ResourceVersion // SHouldn't change, but just in case
			b.Report.Items[i].LastOOM = time.Now()
			glog.V(1).Infof("updating OOMKill information for existing report item %s: %d OOMs, latest OOM %s", item, b.Report.Items[i].NumOOMs, b.Report.Items[i].LastOOM)
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

// MatchItems returns a pointer to a slice of RightSizerReportItem matching the supplied kind,
// namespace, and resource name. IF no items match, nil is returned.
func (b *RightSizerReportBuilder) MatchItems(resourceKind, resourceNamespace, resourceName string) *[]RightSizerReportItem {
	glog.V(2).Infof("starting match items based on kind %s, namespace %s, and name %s", resourceKind, resourceNamespace, resourceName)
	b.itemsLock.RLock()
	defer b.itemsLock.RUnlock()
	var matchedItems []RightSizerReportItem
	for _, item := range b.Report.Items {
		if item.Kind == resourceKind && item.ResourceNamespace == resourceNamespace && item.ResourceName == resourceName {
			matchedItems = append(matchedItems, item)
			glog.V(4).Infof("matched item %v to kind %s, namespace %s, and name %s", item, resourceKind, resourceNamespace, resourceName)
		}
	}
	numMatched := len(matchedItems)
	if numMatched > 0 {
		glog.V(2).Infof("finished match items based on kind %s, namespace %s, and name %s - matched %d", resourceKind, resourceNamespace, resourceName, numMatched)
		return &matchedItems
	}
	glog.V(2).Infof("finished match items based on kind %s, namespace %s, and name %s - matched %d", resourceKind, resourceNamespace, resourceName, numMatched)
	return nil
}

// MatchItemsWithOlderResourceVersion wraps a call to MatchItems(), only
// returning matches with a `ResourceVersion` field lessthan the
// `currentResourceVersion` parameter.
func (b *RightSizerReportBuilder) MatchItemsWithOlderResourceVersion(currentResourceVersion, resourceKind, resourceNamespace, resourceName string) *[]RightSizerReportItem {
	glog.V(2).Infof("starting match items with older ResourceVersion than %q, based on kind %s, namespace %s, and name %s", currentResourceVersion, resourceKind, resourceNamespace, resourceName)
	allMatches := b.MatchItems(resourceKind, resourceNamespace, resourceName)
	if allMatches == nil {
		glog.V(2).Infof("finished match items with older ResourceVersion than %q, based on kind %s, namespace %s, and name %s - no matches regardless of ResourceVersion", currentResourceVersion, resourceKind, resourceNamespace, resourceName)
		return nil
	}
	var olderItems []RightSizerReportItem
	for _, item := range *allMatches {
		if item.ResourceVersion < currentResourceVersion {
			// THis item is older than the current version.
			olderItems = append(olderItems, item)
		}
	}
	numOlderItems := len(olderItems)
	if numOlderItems > 0 {
		glog.V(2).Infof("finished match items with older ResourceVersion than %q, based on kind %s, namespace %s, and name %s - %d matches", currentResourceVersion, resourceKind, resourceNamespace, resourceName, numOlderItems)
		return &olderItems
	}
	glog.V(2).Infof("finished match items with older ResourceVersion than %q, based on kind %s, namespace %s, and name %s - no older ResourceVersions out of %d possible matches", currentResourceVersion, resourceKind, resourceNamespace, resourceName, len(*allMatches))
	return nil
}

// RemoveOldItems removes items from the report that have a last OOMKill older than
// RightSizerReportBuilder.tooOldAge, and returns true if any items were
// removed.
func (b *RightSizerReportBuilder) RemoveOldItems() (anyItemsRemoved bool) {
	glog.V(2).Infoln("starting removing old items based on last OOM kill. . .")
	var finalItems []RightSizerReportItem
	b.itemsLock.Lock()
	defer b.itemsLock.Unlock()
	for _, item := range b.Report.Items {
		if time.Since(item.LastOOM) >= b.tooOldAge {
			glog.V(1).Infof("aging out item %s as its last OOM kill %s is older than %s", item, item.LastOOM, b.tooOldAge)
			// This item is not retained in finalItems
		} else {
			finalItems = append(finalItems, item)
			glog.V(4).Infof("keeping item %s as its last OOM kill %s is not older than %s", item, item.LastOOM, b.tooOldAge)
		}
	}
	if len(finalItems) != len(b.Report.Items) {
		b.Report.Items = finalItems
		anyItemsRemoved = true
	}
	glog.V(2).Infof("finished removing old items based on last OOM kill - removed=%v", anyItemsRemoved)
	return anyItemsRemoved
}

// LoopRemoveOldItems calls RemoveOldItems() every
// minute, and calls RightSizerReportBuilder.WriteConfigMap() if any items were removed.
// This is meant to be run in its own Go ruiteen.
func (b *RightSizerReportBuilder) LoopRemoveOldItems() {
	for range time.Tick(time.Minute * 1) {
		if b.RemoveOldItems() {
			b.WriteConfigMap()
		}
	}
}

// RemoveItems accepts a slice of RightSizerReportItems, and removes them from
// the report, returning true if any items were removed.
// Only the fields included in RightSizerReportItem.String() are compared,
// when matching an item for removal.
func (b *RightSizerReportBuilder) RemoveItems(removeItems []RightSizerReportItem) (anyItemsRemoved bool) {
	numToRemove := len(removeItems)
	if numToRemove == 0 {
		return false
	}
	glog.V(3).Infof("starting removing %d items . . .", numToRemove)
	numStartingItems := len(b.Report.Items)
	finalItems := make([]RightSizerReportItem, numStartingItems)
	copy(finalItems, b.Report.Items)
	b.itemsLock.Lock()
	defer b.itemsLock.Unlock()
	for itemNum := 0; itemNum < len(finalItems); itemNum++ {
		for _, itemToRemove := range removeItems {
			if finalItems[itemNum].String() == itemToRemove.String() {
				glog.V(4).Infof("removing item %s", finalItems[itemNum])
				finalItems = append(finalItems[:itemNum], finalItems[itemNum+1:]...)
				anyItemsRemoved = true
				itemNum-- // re-process this index since an item was just removed
			}
		}
	}
	numFinalItems := len(finalItems)
	if anyItemsRemoved {
		b.Report.Items = finalItems
	}
	glog.V(3).Infof("finished removing %d items - actually removed %d", numToRemove, numStartingItems-numFinalItems)
	return anyItemsRemoved
}

// GetReportJSON( returns the RightSizerReport in JSON form.
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
		glog.Errorf("error while getting report JSON to serve over HTTP: %v", err)
		return
	}
	_, err = w.Write(data)
	if err != nil {
		glog.Errorf("error sending report data over HTTP: %v", err)
	}
}

// RunServer starts the report builder HTTP server and registers the report
// handler. This server can be used to retrieve the current report JSON.
func (b RightSizerReportBuilder) RunServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/report", b.ReportHandler)
	b.HTTPServer.Handler = mux
	inKubernetes := (os.Getenv("KUBERNETES_SERVICE_HOST") != "")
	if inKubernetes {
		b.HTTPServer.Addr = ":8080"
	} else {
		// Use localhost outside of Kubernetes, to avoid Mac OS
		// accept-incoming-network-connection warnings.
		b.HTTPServer.Addr = "localhost:8080"
	}
	go func() { glog.Fatal(b.HTTPServer.ListenAndServe()) }()
}

// WriteConfigMap writes the report to a Kubernetes ConfigMap resource. IF the
// ConfigMap does not exist, it will be created.
func (b *RightSizerReportBuilder) WriteConfigMap() error {
	updateTime := time.Now().String()
	var configMap *core.ConfigMap
	reportBytes, err := b.GetReportJSON()
	if err != nil {
		return err
	}
	reportJSON := string(reportBytes)
	configMaps := b.kubeClient.CoreV1().ConfigMaps(b.configMapNamespaceName)
	configMap, err = configMaps.Get(context.TODO(), b.configMapName, meta.GetOptions{})
	if err != nil && !kube_errors.IsNotFound(err) {
		// This is an unexpected error.
		return fmt.Errorf("unable to get ConfigMap %s/%s to write report: %w", b.configMapNamespaceName, b.configMapName, err)
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
			return fmt.Errorf("unable to update ConfigMap %s/%s to write report: %w", b.configMapNamespaceName, b.configMapName, err)
		}
	}
	if kube_errors.IsNotFound(err) {
		// No ConfigMap found, create one.
		configMap = &core.ConfigMap{
			ObjectMeta: meta.ObjectMeta{
				Namespace: b.configMapNamespaceName,
				Name:      b.configMapName,
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
			return fmt.Errorf("unable to create ConfigMap %s/%s to write report: %w", b.configMapNamespaceName, b.configMapName, err)
		}
	}
	return nil
}

// ReadConfigMap reads the report from a Kubernetes ConfigMap resource. IF
// the ConfigMap does not exist, the report remains unchanged, and a new
// (empty) ConfigMap is created so the Insights Uploader has something to
// read.
// This is useful to populate the report with previous data on Kubernetes controller startup.
func (b *RightSizerReportBuilder) ReadConfigMap() error {
	var configMap *core.ConfigMap
	var reportJSON string
	configMaps := b.kubeClient.CoreV1().ConfigMaps(b.configMapNamespaceName)
	configMap, err := configMaps.Get(context.TODO(), b.configMapName, meta.GetOptions{})
	if kube_errors.IsNotFound(err) {
		// No ConfigMap found; no data to populate the report.
		glog.V(1).Infof("ConfigMap %s/%s does not exist, the report will not be populated with existing state and an empty ConfigMap will be created", b.configMapNamespaceName, b.configMapName)
		b.WriteConfigMap()
		return nil
	}
	if err != nil && !kube_errors.IsNotFound(err) {
		// This is an unexpected error.
		return fmt.Errorf("unable to get ConfigMap %s/%s to read report state: %w", b.configMapNamespaceName, b.configMapName, err)
	}
	if err == nil {
		// Read the report from the ConfigMap
		var ok bool
		reportJSON, ok = configMap.Data["report"]
		if !ok {
			return fmt.Errorf("unable to read report from ConfigMap %s/%s as it does not contain a report key", b.configMapNamespaceName, b.configMapName)
		}
		glog.V(2).Infof("got report JSON from ConfigMap %s/%s: %q", b.configMapNamespaceName, b.configMapName, reportJSON)
		dec := json.NewDecoder(strings.NewReader(reportJSON))
		dec.DisallowUnknownFields() // Return an error if unmarshalling unexpected fields.
		err = dec.Decode(&b.Report)
		if err != nil {
			return fmt.Errorf("unable to unmarshal while reading report from ConfigMap %s/%s: %v", b.configMapNamespaceName, b.configMapName, err)
		}
		glog.V(2).Infof("Read report from ConfigMap %s/%s: %#v", b.configMapNamespaceName, b.configMapName, b.Report)
	}
	return nil
}
