package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
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
		containerMemoryLimits := containerInfo.Resources.Limits.Memory()
		doubledContainerMemoryLimits := containerInfo.Resources.Limits.Memory()
		doubledContainerMemoryLimits.Add(*containerMemoryLimits)
		c.recorder.Eventf(pod, core.EventTypeWarning, "PreviousContainerWasOOMKilled", "The previous instance of the container '%s' (%s) was OOMKilled", s.Name, s.ContainerID)
		ProcessedContainerUpdates.WithLabelValues("oomkilled_event_sent").Inc()

		// Find the owning pod-controller for this pod.
		podControllerObject, err := c.getPodController(pod)
		if err != nil {
			glog.Errorf("unable to get top controller for pod %s/%s: %v", pod.Namespace, pod.Name, err)
		}
		glog.V(1).Infof("Pod %s/%s is owned by pod-controller %s %s", pod.Namespace, pod.Name, podControllerObject.GetKind(), podControllerObject.GetName())
		glog.V(1).Infof("Container %s has memory  limit %v, doubling to %v", containerInfo.Name, containerMemoryLimits, doubledContainerMemoryLimits)
		// Construct a report item.
		var reportItem report.RightSizerReportItem
		reportItem.Kind = podControllerObject.GetKind()
		reportItem.ResourceNamespace = podControllerObject.GetNamespace()
		reportItem.ResourceName = podControllerObject.GetName()
		reportItem.ResourceVersion = podControllerObject.GetResourceVersion()
		reportItem.ResourceContainer = containerInfo.Name
		reportItem.StartingMemory = containerMemoryLimits
		reportItem.EndingMemory = doubledContainerMemoryLimits
		glog.V(1).Infof("Constructed report item: %+v\n", reportItem)
		c.reportBuilder.AddOrUpdateItem(reportItem)
		// Update the state to a ConfigMap.
		err = c.reportBuilder.WriteConfigMap()
		if err != nil {
			glog.Error(err)
		}
		// DOuble memory in-cluster.
		err = c.patchContainerMemoryLimits(podControllerObject, reportItem.ResourceContainer, doubledContainerMemoryLimits)
		if err != nil {
			glog.Errorf("error patching container memory limits: %v", err)
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
	return &topController, nil
}

func (c *Controller) patchContainerMemoryLimits(podController *unstructured.Unstructured, containerName string, newContainerMemoryLimits *resource.Quantity) error {
	glog.V(1).Infof("starting patch %s %s/%s:%s memory limits to %s", podController.GetNamespace(), podController.GetKind(), podController.GetName(), containerName, newContainerMemoryLimits)
	// THis will eventually be a loop to search in different levels of a resource
	// spec.
	podSpecAsInterface, podSpecFound, err := unstructured.NestedMap(podController.UnstructuredContent(), "spec", "template", "spec")
	if err != nil {
		return fmt.Errorf("error finding pod spec in unstructured resource %s %s/%s: %v\n", podController.GetKind(), podController.GetNamespace(), podController.GetName(), err)
	}
	if !podSpecFound {
		return fmt.Errorf("unable to find pod spec in unstructured resource %s %s/%s", podController.GetKind(), podController.GetNamespace(), podController.GetName())
	}
	var podSpec core.PodSpec
	err = runtime.DefaultUnstructuredConverter.
		FromUnstructured(podSpecAsInterface, &podSpec)
	if err != nil {
		fmt.Errorf("error converting podSpec interface %v to a structured pod object: %v", podSpecAsInterface, err)
	}
	var doubledContainerMemoryLimit *resource.Quantity
	var containerNumber string
	var foundContainer bool
	for i, container := range podSpec.Containers {
		// fmt.Printf("container %d name %s has limits %s\n", i, container.Name, container.Resources.Limits.Memory)
		if container.Name == containerName {
			containerNumber = strconv.Itoa(i)
			foundContainer = true
			doubledContainerMemoryLimit = container.Resources.Limits.Memory()
			break
		}
	}
	if !foundContainer {
		return fmt.Errorf("did not find container %s in pod spec %v", containerName, podSpec)
	}
	// until now doubledContainerMemoryLimit is the current limit for the
	// container.
	doubledContainerMemoryLimit.Add(*doubledContainerMemoryLimit)
	glog.V(3).Infof("doubled memory limits for container %s will be %s", containerName, doubledContainerMemoryLimit)
	patch := []interface{}{
		map[string]interface{}{
			"op":    "replace",
			"path":  "/spec/template/spec/containers/" + containerNumber + "/resources/limits/memory",
			"value": newContainerMemoryLimits.String(),
		},
	}
	patchJSON, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("unable to marshal patch %v: %v", patch, err)
	}
	GVK := podController.GroupVersionKind()
	GVKMapping, err := c.RESTMapper.RESTMapping(GVK.GroupKind(), GVK.Version)
	if err != nil {
		return fmt.Errorf("error creating RESTMapper mapping from group-version-kind %v: %v", GVK, err)
	}
	patchClient := c.dynamicClient.Resource(GVKMapping.Resource).Namespace(podController.GetNamespace())
	glog.V(2).Infof("going to patch %s/%s: %#v", podController.GetNamespace(), podController.GetName(), string(patchJSON))
	patchedResource, err := patchClient.Patch(context.TODO(), podController.GetName(), types.JSONPatchType, patchJSON, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("error patching %s %s/%s: %v", podController.GetKind(), podController.GetNamespace(), podController.GetName(), err)
	}
	glog.V(4).Infof("resource after patch is: %v", patchedResource)
	glog.V(1).Infof("finished patch %s %s/%s:%s memory limits to %s", podController.GetNamespace(), podController.GetKind(), podController.GetName(), containerName, newContainerMemoryLimits)
	return nil
}
