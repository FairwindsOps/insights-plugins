package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	fwControllerUtils "github.com/fairwindsops/controller-utils/pkg/controller"
	"github.com/fairwindsops/insights-plugins/right-sizer/src/report"
	"github.com/fairwindsops/insights-plugins/right-sizer/src/util"
	"github.com/golang/glog"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

const (
	// informerSyncMinute defines how often the cache is synced from Kubernetes
	informerSyncMinute = 2
	// TerminationReasonOOMKilled is the reason of a ContainerStateTerminated that reflects an OOM kill
	TerminationReasonOOMKilled = "OOMKilled"
)

// Controller is a controller that listens on Pod changes and create Kubernetes Events
// when a container reports it was previously killed
type Controller struct {
	Stop           chan struct{}
	client         kubernetes.Interface
	dynamicClient  dynamic.Interface // used to find owning pod-controller
	k8sFactory     informers.SharedInformerFactory
	podLister      util.PodLister
	recorder       record.EventRecorder
	startTime      time.Time
	stopCh         chan struct{}
	eventAddedCh   chan *core.Event
	eventUpdatedCh chan *eventUpdateGroup
	RESTMapper     meta.RESTMapper // used to find owning pod-controller
	reportBuilder  *report.RightSizerReportBuilder
}

type eventUpdateGroup struct {
	oldEvent *core.Event
	newEvent *core.Event
}

// NewController returns an instance of the Controller
func NewController(stop chan struct{}) *Controller {
	client, dynamicClient, RESTMapper := util.Clientset()
	k8sFactory := informers.NewSharedInformerFactory(client, time.Minute*informerSyncMinute)

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: client.CoreV1().Events("")})

	controller := &Controller{
		stopCh:         make(chan struct{}),
		Stop:           stop,
		k8sFactory:     k8sFactory,
		client:         client,
		dynamicClient:  dynamicClient,
		RESTMapper:     RESTMapper,
		podLister:      k8sFactory.Core().V1().Pods().Lister(),
		eventAddedCh:   make(chan *core.Event),
		eventUpdatedCh: make(chan *eventUpdateGroup),
		recorder:       eventBroadcaster.NewRecorder(scheme.Scheme, core.EventSource{Component: "oom-event-generator"}),
		startTime:      time.Now(),
		reportBuilder:  report.NewRightSizerReportBuilder(client),
	}

	eventsInformer := informers.SharedInformerFactory(k8sFactory).Core().V1().Events().Informer()
	eventsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			controller.eventAddedCh <- obj.(*core.Event)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			controller.eventUpdatedCh <- &eventUpdateGroup{
				oldEvent: oldObj.(*core.Event),
				newEvent: newObj.(*core.Event),
			}
		},
	})

	return controller
}

// Run is the main loop that processes Kubernetes Pod changes
func (c *Controller) Run() error {
	err := c.reportBuilder.ReadConfigMap()
	if err != nil {
		glog.Errorf("while attempting to read state from ConfigMap: %v", err)
	}
	go c.reportBuilder.LoopRemoveOldItems() // age out items
	c.reportBuilder.RunServer()             // Run an HTTP server to serve the current report.
	c.k8sFactory.Start(c.stopCh)
	c.k8sFactory.WaitForCacheSync(c.Stop)

	for {
		select {
		case event := <-c.eventAddedCh:
			c.evaluateEvent(event)
		case eventUpdate := <-c.eventUpdatedCh:
			c.evaluateEventUpdate(eventUpdate)
		case <-c.Stop:
			glog.Info("Stopping")
			return nil
		}

	}
}

const startedEvent = "Started"
const podKind = "Pod"

func isContainerStartedEvent(event *core.Event) bool {
	return (event.Reason == startedEvent &&
		event.InvolvedObject.Kind == podKind)
}

func isSameEventOccurrence(g *eventUpdateGroup) bool {
	return (g.oldEvent.InvolvedObject == g.newEvent.InvolvedObject &&
		g.oldEvent.Count == g.newEvent.Count)
}

func (c *Controller) evaluateEvent(event *core.Event) {
	glog.V(2).Infof("got event %s/%s (count: %d), reason: %s, involved object: %s", event.ObjectMeta.Namespace, event.ObjectMeta.Name, event.Count, event.Reason, event.InvolvedObject.Kind)
	if !isContainerStartedEvent(event) {
		// IF this update matches a kind/namespace/name of a pod-controller in the
		// report, remove related report items.
		relatedReportItems := c.reportBuilder.MatchItemsWithOlderResourceVersion(event.InvolvedObject.ResourceVersion, event.InvolvedObject.Kind, event.InvolvedObject.Namespace, event.InvolvedObject.Name)
		if relatedReportItems != nil {
			eventSummary := fmt.Sprintf("%s %s/%s %s", event.Reason, event.InvolvedObject.Kind, event.InvolvedObject.Namespace, event.InvolvedObject.Name)
			glog.V(1).Infof("going to remove report items related to event %q: %v", eventSummary, relatedReportItems)
			if c.reportBuilder.RemoveItems(*relatedReportItems) {
				c.reportBuilder.WriteConfigMap()
			}
		}
		return
	}
	pod, err := c.podLister.Pods(event.InvolvedObject.Namespace).Get(event.InvolvedObject.Name)
	if err != nil {
		glog.Errorf("Failed to retrieve pod %s/%s, due to: %v", event.InvolvedObject.Namespace, event.InvolvedObject.Name, err)
		return
	}
	c.evaluatePodStatus(pod)
}

func (c *Controller) evaluateEventUpdate(eventUpdate *eventUpdateGroup) {
	event := eventUpdate.newEvent
	if eventUpdate.oldEvent == nil {
		glog.V(4).Infof("No old event present for event %s/%s (count: %d), reason: %s, involved object: %s, skipping processing", event.ObjectMeta.Namespace, event.ObjectMeta.Name, event.Count, event.Reason, event.InvolvedObject.Kind)
		return
	}
	if reflect.DeepEqual(eventUpdate.oldEvent, eventUpdate.newEvent) {
		glog.V(4).Infof("Event %s/%s (count: %d), reason: %s, involved object: %s, did not change: skipping processing", event.ObjectMeta.Namespace, event.ObjectMeta.Name, event.Count, event.Reason, event.InvolvedObject.Kind)
		return
	}
	if isSameEventOccurrence(eventUpdate) {
		glog.V(3).Infof("Event %s/%s (count: %d), reason: %s, involved object: %s, did not change wrt. to restart count: skipping processing", eventUpdate.newEvent.ObjectMeta.Namespace, eventUpdate.newEvent.ObjectMeta.Name, eventUpdate.newEvent.Count, eventUpdate.newEvent.Reason, eventUpdate.newEvent.InvolvedObject.Kind)
		return
	}
	if !isContainerStartedEvent(event) {
		// IF this update matches a kind/namespace/name of a pod-controller in the
		// report, remove related report items.
		relatedReportItems := c.reportBuilder.MatchItemsWithOlderResourceVersion(event.InvolvedObject.ResourceVersion, event.InvolvedObject.Kind, event.InvolvedObject.Namespace, event.InvolvedObject.Name)
		if relatedReportItems != nil {
			eventSummary := fmt.Sprintf("%s %s/%s %s", event.Reason, event.InvolvedObject.Kind, event.InvolvedObject.Namespace, event.InvolvedObject.Name)
			glog.V(1).Infof("going to remove report items related to event %q: %v", eventSummary, relatedReportItems)
			if c.reportBuilder.RemoveItems(*relatedReportItems) {
				c.reportBuilder.WriteConfigMap()
			}
		}
		return
	}
	pod, err := c.podLister.Pods(event.InvolvedObject.Namespace).Get(event.InvolvedObject.Name)
	if err != nil {
		glog.Errorf("Failed to retrieve pod %s/%s, due to: %v", event.InvolvedObject.Namespace, event.InvolvedObject.Name, err)
		return
	}
	c.evaluatePodStatus(pod)
}

func (c *Controller) evaluatePodStatus(pod *core.Pod) {
	// Look for OOMKilled containers
	for _, s := range pod.Status.ContainerStatuses {
		if s.LastTerminationState.Terminated == nil || s.LastTerminationState.Terminated.Reason != TerminationReasonOOMKilled {
			ProcessedContainerUpdates.WithLabelValues("not_oomkilled").Inc()
			continue
		}

		if s.LastTerminationState.Terminated.FinishedAt.Time.Before(c.startTime) {
			glog.V(1).Infof("The container '%s' in '%s/%s' was terminated before this controller started", s.Name, pod.Namespace, pod.Name)
			ProcessedContainerUpdates.WithLabelValues("oomkilled_termination_too_old").Inc()
			continue
		}
		// Get information about the container that was OOM-killed.
		var containerInfo core.Container
		for _, container := range pod.Spec.Containers {
			if container.Name == s.Name { // matched from the event
				containerInfo = container
				break
			}
		}
		containerMemoryLimit := containerInfo.Resources.Limits.Memory()
		// This `doubled limit` code is incomplete, and only serves to test
		// calculating a higher limit for the future.
		doubledContainerMemoryLimit := containerInfo.Resources.Limits.Memory()
		doubledContainerMemoryLimit.Add(*containerMemoryLimit)
		c.recorder.Eventf(pod, core.EventTypeWarning, "PreviousContainerWasOOMKilled", "The previous instance of the container '%s' (%s) was OOMKilled", s.Name, s.ContainerID)
		ProcessedContainerUpdates.WithLabelValues("oomkilled_event_sent").Inc()

		// Find the owning pod-controller for this pod.
		podControllerObject, err := c.getPodController(pod)
		if err != nil {
			glog.Errorf("unable to get top controller for pod %s/%s: %v", pod.Namespace, pod.Name, err)
		}
		glog.V(1).Infof("Pod %s/%s is owned by pod-controller %s %s", pod.Namespace, pod.Name, podControllerObject.GetKind(), podControllerObject.GetName())
		glog.V(1).Infof("Container %s has memory  limit %v, if we doubled that it would be %v", containerInfo.Name, containerMemoryLimit, doubledContainerMemoryLimit)
		// Construct a report item.
		var reportItem report.RightSizerReportItem
		reportItem.Kind = podControllerObject.GetKind()
		reportItem.ResourceNamespace = podControllerObject.GetNamespace()
		reportItem.ResourceName = podControllerObject.GetName()
		reportItem.ResourceVersion = podControllerObject.GetResourceVersion()
		reportItem.ResourceContainer = containerInfo.Name
		reportItem.StartingMemory = containerMemoryLimit
		reportItem.EndingMemory = containerMemoryLimit // same as limit for now
		glog.V(1).Infof("Constructed report item: %+v\n", reportItem)
		c.reportBuilder.AddOrUpdateItem(reportItem)
		// Update the state to a ConfigMap.
		err = c.reportBuilder.WriteConfigMap()
		if err != nil {
			glog.Error(err)
		}
	}
}

// GetPodController accepts a typed pod object, and returns the pod-controller
// which owns the pod.
// E.G. an owning pod-controller might be a Kubernetes Deployment, DaemonSet,
// or CronJob.
func (c *Controller) getPodController(pod *core.Pod) (*unstructured.Unstructured, error) {
	// Convert a pod type to an unstructured one.
	podJSON, err := json.Marshal(pod)
	if err != nil {
		return nil, err
	}
	objectAsMap := make(map[string]interface{})
	err = json.Unmarshal(podJSON, &objectAsMap)
	if err != nil {
		return nil, err
	}
	unstructuredPod := unstructured.Unstructured{
		Object: objectAsMap,
	}

	topController, err := fwControllerUtils.GetTopController(context.TODO(), c.dynamicClient, c.RESTMapper, unstructuredPod, nil)
	if err != nil {
		return nil, err
	}
	glog.V(2).Infof("found controller kind %q named %q", topController.GetKind(), topController.GetName())
	// begin temporary code to double limits

	// podSpecInterface := fwControllerUtils.GetPodSpec(topController.UnstructuredContent())
	podSpecInterface, found, err := unstructured.NestedMap(topController.UnstructuredContent(), "spec", "template", "spec")
	if err != nil {
		fmt.Printf("error finding pod spec in unstructured top controller: %v\n", err)
	}
	if !found {
		fmt.Println("pod spec not found in unstructured top controller")
	}
	fmt.Printf("PodSpec interface is %v\n", podSpecInterface)
	var podSpec core.PodSpec
	err = runtime.DefaultUnstructuredConverter.
		FromUnstructured(podSpecInterface, &podSpec)
	if err != nil {
		fmt.Printf("error converting podSpec interface to pod: %v, err")
	}
	var doubledContainerMemoryLimit *resource.Quantity
	for i, container := range podSpec.Containers {
		fmt.Printf("container %d name %s has limits %s\n", i, container.Name, container.Resources.Limits.Memory)
		doubledContainerMemoryLimit = container.Resources.Limits.Memory()
		doubledContainerMemoryLimit.Add(*doubledContainerMemoryLimit)
		fmt.Printf("Doubled limits is %s\n", doubledContainerMemoryLimit)
	}
	patch := []interface{}{
		map[string]interface{}{
			"op":    "replace",
			"path":  "/spec/template/spec/containers/0/resources/limits/memory",
			"value": doubledContainerMemoryLimit.String(),
		},
	}
	patchJSON, err := json.Marshal(patch)
	if err != nil {
		fmt.Printf("Unable to marshal patch JSON: %v\n", err)
	}

	// Create resources we need to patch the topController.
	GVK := topController.GroupVersionKind()
	fmt.Printf("group version kind is: %v\n", GVK)
	mapping, err := c.RESTMapper.RESTMapping(GVK.GroupKind(), GVK.Version)
	if err != nil {
		fmt.Printf("error getting mapping from GVK: %v\n", err)
	}
	fmt.Printf("Showing mapping: %v\n", mapping)
	GVR := schema.GroupVersionResource{
		Group:    GVK.Group,
		Version:  GVK.Version,
		Resource: strings.ToLower(GVK.Kind) + "s",
	}
	fmt.Printf("The group-version-resource  is: %v\n", GVR)
	patchClient := c.dynamicClient.Resource(mapping.Resource).Namespace(topController.GetNamespace())
	// patchClient := c.dynamicClient.Resource(GVR).Namespace(topController.GetNamespace())
	fmt.Printf("Going to patch namespace %s and name %s with: %#v\n", topController.GetNamespace(), topController.GetName(), string(patchJSON))
	_, err = patchClient.Patch(context.TODO(), topController.GetName(), types.JSONPatchType, patchJSON, metav1.PatchOptions{})
	if err != nil {
		fmt.Printf("Error patching: %v\n", err)
	}

	// marker
	// dynamicClient.Resource(mapping.Resource).Namespace(namespace).List(ctx, metav1.ListOptions{})
	return &topController, nil
}
