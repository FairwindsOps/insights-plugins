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

	"github.com/fairwindsops/insights-plugins/right-sizer/src/util"
	"github.com/golang/glog"
	core "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	kube_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// RightSizerReportItem contains properties of a right-sizer item.
type RightSizerReportItem struct {
	Kind               string             `json:"kind"`
	ResourceName       string             `json:"resourceName"`
	ResourceNamespace  string             `json:"resourceNamespace"`
	ResourceContainer  string             `json:"resourceContainer"`
	StartingMemory     *resource.Quantity `json:"startingMemory"`
	EndingMemory       *resource.Quantity `json:"endingMemory"`
	NumOOMs            int64              `json:"numOOMs"`
	FirstOOM           time.Time          `json:"firstOOM"`
	LastOOM            time.Time          `json:"lastOOM"`
	ResourceVersion    string             `json:"ResourceVersion"`
	ResourceGeneration int64              `json:"resourceGeneration"`
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
	memoryLimitMultiplier                 float64       // The multiplier used to increase container memory limits.
	tooOldAge                             time.Duration // When an item should be removed based on its age
	configMapNamespaceName, configMapName string        // ConfigMap name to persist report state
	HTTPServer                            *http.Server  // Allows retrieving the current report
	kubeClientResources                   util.KubeClientResources
	// marker
	// kubeClient                            kubernetes.Interface // Used to read/write state ConfigMap
}

// String represents unique fields to differentiate a report item.
func (i RightSizerReportItem) String() string {
	return fmt.Sprintf("%s %s/%s:%s", i.Kind, i.ResourceNamespace, i.ResourceName, i.ResourceContainer)
}

// MaxAllowedEndingMemory returns the highest value that memory limits are allowed to
// be increased. A multiplier for RightSizerReportItem.StartingMemory is
// currently hard-coded into this function.
func (i RightSizerReportItem) MaxAllowedEndingMemory() (*resource.Quantity, error) {
	max, err := util.MultiplyResourceQuantity(i.StartingMemory, 2.0)
	if err != nil {
		return max, fmt.Errorf("while determining maximum memory limits: %w", err)
	}
	return max, nil
}

// NewRightSizerReportBuilder returns a pointer to a new initialized
// RightSizerReportBuilder type.
// TODO: populate defaults (state ConfigMap, too-old-age) from CLI flags.
func NewRightSizerReportBuilder(kubeClientResources util.KubeClientResources) *RightSizerReportBuilder {
	// func NewRightSizerReportBuilder(kubeClient kubernetes.Interface) *RightSizerReportBuilder {
	b := &RightSizerReportBuilder{
		Report:              &RightSizerReportProperties{},
		itemsLock:           &sync.RWMutex{},
		kubeClientResources: kubeClientResources,
		tooOldAge:           8.64e+13, // 24 HRs
		// tooOldAge: 6e+10, // 1 minute
		// tooOldAge:              3e+11, // 5 minutes
		memoryLimitMultiplier:  1.2,
		configMapNamespaceName: "insights-agent",
		configMapName:          "insights-agent-right-sizer-controller-state",
		HTTPServer: &http.Server{
			ReadTimeout:  2500 * time.Millisecond, // time to read request headers and   optionally body
			WriteTimeout: 10 * time.Second,
		},
	}
	return b
}

// GetMemoryLimitsMultiplier returns the configured multiplier, the multiplier
// by which container memory limits are increased.
func (b *RightSizerReportBuilder) GetMemoryLimitsMultiplier() float64 {
	return b.memoryLimitMultiplier
}

func (b *RightSizerReportBuilder) PopulateExistingItemFields(yourItem *RightSizerReportItem) (foundItem bool) {
	b.itemsLock.RLock()
	defer b.itemsLock.RUnlock()
	for _, item := range b.Report.Items {
		if item.Kind == yourItem.Kind && item.ResourceNamespace == yourItem.ResourceNamespace && item.ResourceName == yourItem.ResourceName && item.ResourceContainer == yourItem.ResourceContainer {
			foundItem = true
			*yourItem = item
			return // uses the named arguments in the func definition
		}
	}
	return // uses the named arguments in the func definition
}

// AddOrUpdateItem accepts a RightSizerReportItem and adds or updates it
// in the report.
// If the item already exists, `NumOOMs` and `LastOOM` fields are updated.
func (b *RightSizerReportBuilder) AddOrUpdateItem(newItem RightSizerReportItem) {
	b.itemsLock.Lock()
	defer b.itemsLock.Unlock()
	for i, item := range b.Report.Items {
		if item.Kind == newItem.Kind && item.ResourceNamespace == newItem.ResourceNamespace && item.ResourceName == newItem.ResourceName && item.ResourceContainer == newItem.ResourceContainer {
			// Update the existing item.
			b.Report.Items[i].NumOOMs++
			b.Report.Items[i].LastOOM = time.Now()
			// The entire newItem is not updated in the report, because some fields need to
			// be retain from the original item, such as FirstOOM and StartingMemory.
			// UPdate fields that the controller will have set, in the new item.
			b.Report.Items[i].ResourceVersion = newItem.ResourceVersion
			b.Report.Items[i].ResourceGeneration = newItem.ResourceGeneration
			b.Report.Items[i].EndingMemory = newItem.EndingMemory
			glog.V(1).Infof("updating OOMKill information for existing report item %#v", b.Report.Items[i])
			return
		}
	}
	// This item is new to this report.
	newItem.NumOOMs = 1
	newItem.FirstOOM = time.Now()
	newItem.LastOOM = newItem.FirstOOM
	b.Report.Items = append(b.Report.Items, newItem)
	glog.V(1).Infof("adding new report item %v", newItem)
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

// MatchItemsOlderWithModifiedMemoryLimits wraps a call to MatchItems(), only
// returning matches where items have an older ResourceGeneration than the `metadata.generation` field of the current
// in-cluster resource, and whos memory limits are different from last
// seen/updated in our report.
// The current resource is fetched from Kube via core/v1.ObjectReference, typically
// provided along with a Kube Event.
func (b *RightSizerReportBuilder) MatchItemsOlderWithModifiedMemoryLimits(involvedObject core.ObjectReference) (*[]RightSizerReportItem, error) {
	glog.V(2).Infof("starting match items with older metadata.generation and different memory limits than the in-cluster resource, based on kind %s, namespace %s, and name %s", involvedObject.Kind, involvedObject.Namespace, involvedObject.Name)
	allMatches := b.MatchItems(involvedObject.Kind, involvedObject.Namespace, involvedObject.Name)
	if allMatches == nil {
		glog.V(2).Infof("finished match items with older metadata.generation and different memory limits than in-cluster, based on kind %s, namespace %s, and name %s - no matches regardless of generation", involvedObject.Kind, involvedObject.Namespace, involvedObject.Name)
		return nil, nil
	}
	currentResource, foundResource, err := b.GetResourceFromInvolvedObject(involvedObject)
	if !foundResource {
		// Since there is no generation in-cluster to compare, return no matches
		// This could mean that the resource has been deleted from the cluster, but
		// we'll allow the separate time-based aging out of report items to handle
		// this case for now.
		glog.V(2).Infof("finished match items with older metadata.generation and different memory limits than the in-cluster resource, based on kind %s, namespace %s, and name %s - no resource was found in-cluster", involvedObject.Kind, involvedObject.Namespace, involvedObject.Name)
		return nil, nil
	}
	if err != nil {
		glog.Errorf("error getting in-cluster resource to match items with older metadata.generation and different memory limits than the in-cluster resource, based on kind %s, namespace %s, and name %s: %v", involvedObject.Kind, involvedObject.Namespace, involvedObject.Name, err)
		return nil, fmt.Errorf("error finding resource from involved object %s %s/%s: %v", involvedObject.Kind, involvedObject.Namespace, involvedObject.Name, err)
	}
	currentGeneration := currentResource.GetGeneration()
	glog.V(4).Infof("got in-cluster generation %d, while matching items with older metadata.generation and different memory limits than the in-cluster resource, based on kind %s, namespace %s, and name %s", currentGeneration, involvedObject.Kind, involvedObject.Namespace, involvedObject.Name)
	if currentGeneration < 1 {
		glog.V(2).Infof("finished match items with older metadata.generation and different memory limits than the in-cluster resource, based on kind %s, namespace %s, and name %s - the resource has no generation", involvedObject.Kind, involvedObject.Namespace, involvedObject.Name)
	}
	var olderItems []RightSizerReportItem
	for _, item := range *allMatches {
		if item.ResourceGeneration < 1 {
			glog.Errorf("skipping matching report item as older with different memory limits item - item has no ResourceGeneration: %#v", item)
			continue
		}
		currentPodSpec, _, err := util.FindPodSpec(currentResource)
		if err != nil {
			glog.Errorf("skipping matching report item %q as older with different memory limits - error finding pod spec in unstructured resource %s %s/%s: %v", item, currentResource.GetKind(), currentResource.GetNamespace(), currentResource.GetName(), err)
			continue
		}
		currentContainer, _, foundContainer := util.FindContainerInPodSpec(currentPodSpec, item.ResourceContainer)
		if !foundContainer {
			glog.Errorf("skipping matching report item %q as older with different memory limits - error finding container %s in pod-spec: %v", item.ResourceContainer, currentPodSpec)
			continue
		}
		currentMemoryLimits := *currentContainer.Resources.Limits.Memory()
		MLEquality := item.EndingMemory.Cmp(currentMemoryLimits) // will be 0 if both resource.Quantity values match.
		if item.ResourceGeneration < currentGeneration && MLEquality != 0 {
			// THis item is older, or has different memory limits, than the
			// current in-cluster version.
			olderItems = append(olderItems, item)
			glog.V(2).Infof("report item is older or has different memory limits than the in-cluster resource: MLEquality is %d (-1=report < in-cluster or 1=report > in-cluster), in-cluster has memory limits %s and generation %d, report item has memory limits %s and generation %d", MLEquality, currentMemoryLimits, currentGeneration, item.EndingMemory, item.ResourceGeneration)
		}
	}
	numOlderItems := len(olderItems)
	if numOlderItems > 0 {
		glog.V(2).Infof("finished match items with older metadata.generation than %d and different memory limits, based on kind %s, namespace %s, and name %s - %d matches", currentGeneration, involvedObject.Kind, involvedObject.Namespace, involvedObject.Name, numOlderItems)
		return &olderItems, nil
	}
	glog.V(2).Infof("finished match items with older metadata.generation than %d and different memory limits, based on kind %s, namespace %s, and name %s - no older generations out of %d possible matches", currentGeneration, involvedObject.Kind, involvedObject.Namespace, involvedObject.Name, len(*allMatches))
	return nil, nil
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
		if b.tooOldAge > 0 && time.Since(item.LastOOM) >= b.tooOldAge {
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
	inKube := os.Getenv("KUBERNETES_SERVICE_HOST")
	if inKube != "" {
		b.HTTPServer.Addr = "0.0.0.0:8080"
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
	configMaps := b.kubeClientResources.Client.CoreV1().ConfigMaps(b.configMapNamespaceName)
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
	configMaps := b.kubeClientResources.Client.CoreV1().ConfigMaps(b.configMapNamespaceName)
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

// GetResourceFromInvolvedObject fetches a resource from in-cluster, based on
// the core.ObjectReference, which is typically included in a Kube Event.
// The primary reason this function is in this package, is to use a Kube
// dynamic client and RESTMapper, without them having to be parameters.
func (b *RightSizerReportBuilder) GetResourceFromInvolvedObject(involvedObject core.ObjectReference) (resource *unstructured.Unstructured, found bool, err error) {
	// A GroupVersionKind is required to create the RESTMapping, which maps;
	// converts the API version and kind to the correct capitolization and plural
	// syntax required by the Kube API.
	GVK := involvedObject.GroupVersionKind()
	GVKMapping, err := b.kubeClientResources.RESTMapper.RESTMapping(GVK.GroupKind(), GVK.Version)
	if err != nil {
		return nil, false, fmt.Errorf("error creating RESTMapper mapping from group-version-kind %v: %v", GVK, err)
	}
	getterClient := b.kubeClientResources.DynamicClient.Resource(GVKMapping.Resource).Namespace(involvedObject.Namespace)
	glog.V(2).Infof("going to fetch resource %s %s/%s", involvedObject.Kind, involvedObject.Namespace, involvedObject.Name)
	resource, err = getterClient.Get(context.TODO(), involvedObject.Name, metav1.GetOptions{})
	if k8sErrors.IsNotFound(err) {
		return nil, false, nil // not found is not a true error
	}
	if err != nil {
		return nil, false, err
	}
	return resource, true, nil
}
