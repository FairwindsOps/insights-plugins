package controller

import (
	"fmt"
	"reflect"
	"time"

	"github.com/fairwindsops/insights-plugins/right-sizer/src/report"
	"github.com/fairwindsops/insights-plugins/right-sizer/src/util"
	"github.com/golang/glog"
	core "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/informers"
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

// Controller is a controller that listens for Pod OOM-kill events and changes
// to owning pod-controllers, optionally changing memory limits of the
// pod-controller.
type Controller struct {
	kubeClientResources util.KubeClientResources
	config              controllerConfig
	Stop                chan struct{}
	k8sFactory          informers.SharedInformerFactory
	podLister           util.PodLister
	recorder            record.EventRecorder
	startTime           time.Time
	stopCh              chan struct{}
	eventAddedCh        chan *core.Event
	eventUpdatedCh      chan *eventUpdateGroup
	reportBuilder       *report.RightSizerReportBuilder
}

// ControllerConfig represents configurable options for the controller.
type controllerConfig struct {
	updateMemoryLimits           bool
	updateMemoryLimitsMultiplier float64
	maxMemoryLimitsUpdateFactor  float64
	updateMemoryMinimumOOMs      int64
	allowedNamespaces            []string // Allowed namespaces for all operations (alert, updating limits).
}

type eventUpdateGroup struct {
	oldEvent *core.Event
	newEvent *core.Event
}

// controllerOption specifies controllerConfig fields as functions.
// THis is the "functional options" pattern, allowing the NewController() constructor to use something like "named parameters," and for those parameters to be optional.
type controllerOption func(*controllerConfig)

// WithAllowedNamespaces sets the corresponding field in a controllerConfig type.
func WithAllowedNamespaces(value []string) controllerOption {
	return func(c *controllerConfig) {
		c.allowedNamespaces = value
		return
	}
}

// WithUpdateMemoryLimits sets the corresponding field in a controllerConfig type.
func WithUpdateMemoryLimits(value bool) controllerOption {
	return func(c *controllerConfig) {
		c.updateMemoryLimits = value
		return
	}
}

// WithUpdateMemoryLimitsMultiplier sets the corresponding field in a controllerConfig type.
func WithUpdateMemoryLimitsMultiplier(value float64) controllerOption {
	return func(c *controllerConfig) {
		if value > 0.0 {
			c.updateMemoryLimitsMultiplier = value
		}
		return
	}
}

// WithUpdateMemoryMinimumOOMs sets the corresponding field in a controllerConfig type.
func WithUpdateMemoryMinimumOOMs(value int64) controllerOption {
	return func(c *controllerConfig) {
		if value > 0 {
			c.updateMemoryMinimumOOMs = value
		}
		return
	}
}

// WithMaxMemoryLimitsUpdateFactor sets the corresponding field in a
// controllerConfig type.
func WithMaxMemoryLimitsUpdateFactor(value float64) controllerOption {
	return func(c *controllerConfig) {
		if value > 0.0 {
			c.maxMemoryLimitsUpdateFactor = value
		}
		return
	}
}

// NewController returns an instance of the Controller
func NewController(stop chan struct{}, kubeClientResources util.KubeClientResources, rb *report.RightSizerReportBuilder, options ...controllerOption) *Controller {
	cfg := &controllerConfig{
		updateMemoryLimits:           false,
		updateMemoryMinimumOOMs:      2,
		updateMemoryLimitsMultiplier: 1.2,
		maxMemoryLimitsUpdateFactor:  2.0,
	}

	// Process functional options.
	for _, o := range options {
		o(cfg)
	}

	k8sFactory := informers.NewSharedInformerFactory(kubeClientResources.Client, time.Minute*informerSyncMinute)

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: kubeClientResources.Client.CoreV1().Events("")})

	controller := &Controller{
		stopCh:              make(chan struct{}),
		Stop:                stop,
		k8sFactory:          k8sFactory,
		kubeClientResources: kubeClientResources,
		config:              *cfg,
		podLister:           k8sFactory.Core().V1().Pods().Lister(),
		eventAddedCh:        make(chan *core.Event),
		eventUpdatedCh:      make(chan *eventUpdateGroup),
		recorder:            eventBroadcaster.NewRecorder(scheme.Scheme, core.EventSource{Component: "oom-event-generator"}),
		startTime:           time.Now(),
		reportBuilder:       rb,
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

// evaluateEvent processes a Kubernetes event, including add/update/delete of
// related report items.
func (c *Controller) evaluateEvent(event *core.Event) {
	glog.V(4).Infof("got event %s/%s (count: %d), reason: %s, involved object: %s", event.ObjectMeta.Namespace, event.ObjectMeta.Name, event.Count, event.Reason, event.InvolvedObject.Kind)
	if len(c.config.allowedNamespaces) > 0 && !util.Contains(c.config.allowedNamespaces, event.ObjectMeta.Namespace) {
		glog.V(4).Infof("ignoring event %s/%s as its namespace is not allowed", event.ObjectMeta.Namespace, event.ObjectMeta.Name)
		return
	}
	if !isContainerStartedEvent(event) {
		// IF this update matches a kind/namespace/name of a pod-controller in the
		// report, remove related report items.
		// relatedReportItems := c.reportBuilder.MatchItemsWithOlderResourceVersion(event.InvolvedObject.ResourceVersion, event.InvolvedObject.Kind, event.InvolvedObject.Namespace, event.InvolvedObject.Name)
		relatedReportItems, err := c.reportBuilder.MatchItemsOlderWithModifiedMemoryLimits(event.InvolvedObject)
		if err != nil {
			glog.Errorf("error getting related report items: %w", err)
		}
		if relatedReportItems != nil {
			eventSummary := fmt.Sprintf("%s %s/%s %s", event.Reason, event.InvolvedObject.Kind, event.InvolvedObject.Namespace, event.InvolvedObject.Name)
			glog.V(1).Infof("going to remove these report items, triggered by the related event %q: %#v", eventSummary, relatedReportItems)
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

// evaluateEventUpdate is a wrapper around evaluateEvent, which first verifies
// an event is not a duplicate.
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
	c.evaluateEvent(event)
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
		c.recorder.Eventf(pod, core.EventTypeWarning, "PreviousContainerWasOOMKilled", "The previous instance of the container '%s' (%s) was OOMKilled", s.Name, s.ContainerID)
		ProcessedContainerUpdates.WithLabelValues("oomkilled_event_sent").Inc()

		// Find the owning pod-controller for this pod.
		podControllerObject, err := util.GetControllerFromPod(c.kubeClientResources, pod)
		if err != nil {
			glog.Errorf("unable to get top controller for pod %s/%s: %v", pod.Namespace, pod.Name, err)
		}
		glog.V(1).Infof("Pod %s/%s is owned by pod-controller %s %s", pod.Namespace, pod.Name, podControllerObject.GetKind(), podControllerObject.GetName())
		// Start constructing a report item
		var reportItem report.RightSizerReportItem
		reportItem.Kind = podControllerObject.GetKind()
		reportItem.ResourceNamespace = podControllerObject.GetNamespace()
		reportItem.ResourceName = podControllerObject.GetName()
		reportItem.ResourceContainer = containerInfo.Name
		itemAlreadyExists := c.reportBuilder.PopulateExistingItemFields(&reportItem) // Get StartingMemory, Etc.
		if !itemAlreadyExists {
			reportItem.StartingMemory = containerMemoryLimits
		}
		reportItem.NumOOMs++
		reportItem.EndingMemory = containerMemoryLimits // equals Starting Memory unless limits are to be updated.
		reportItem.LastOOM = time.Now()
		reportItem.ResourceVersion = podControllerObject.GetResourceVersion()
		// marker
		generation, generationMatched, err := unstructured.NestedInt64(podControllerObject.UnstructuredContent(), "metadata", "generation")
		if !generationMatched {
			fmt.Printf("did not match generation\n")
		}
		if err != nil {
			fmt.Printf("error matching generation: %v\n", err)
		}
		fmt.Printf("generation is %d\n", generation)
		reportItem.ResourceGeneration = generation // at the time the OOM-kill is seen.
		if c.config.updateMemoryLimits {
			// Increase memory limits in-cluster.
			if c.config.updateMemoryMinimumOOMs > 0 && reportItem.NumOOMs >= c.config.updateMemoryMinimumOOMs {
				newContainerMemoryLimits, newConversionErr := util.MultiplyResourceQuantity(containerMemoryLimits, c.config.updateMemoryLimitsMultiplier)
				if newConversionErr != nil {
					glog.Errorf("error multiplying new memory limits for %s - memory limits cannot be updated: %v", reportItem, err)
				}
				maxAllowedLimits, maxConversionErr := reportItem.MaxAllowedEndingMemory(c.config.maxMemoryLimitsUpdateFactor)
				if maxConversionErr != nil {
					glog.Errorf("error multiplying maximum allowed memory limits for %s - memory limits cannot be updated: %v", reportItem, err)
				}
				MLEquality := newContainerMemoryLimits.Cmp(*maxAllowedLimits) // will be -1 if max<new, 1 if max>new
				glog.V(4).Infof("calculated new memory limits are %s, max allowed is %s, limit comparison is %d (-1=new<max or 1=new>max), for report item %s", newContainerMemoryLimits, maxAllowedLimits, MLEquality, reportItem)
				// Besides being a good sanity check, using IsZero validates there were no
				// conversion errors above.
				if !newContainerMemoryLimits.IsZero() && MLEquality <= 0 {
					glog.V(1).Infof("%s has memory  limit %v, updating to %v", reportItem, containerMemoryLimits, newContainerMemoryLimits)
					patchedResource, err := util.PatchContainerMemoryLimits(c.kubeClientResources, podControllerObject, reportItem.ResourceContainer, newContainerMemoryLimits)
					if err != nil {
						// EndingMemory remains the previously set StartingMemory value.
						glog.Errorf("error patching %s memory limits: %v", reportItem, err)
					} else {
						// The post-patch memory limits may be different than the limits in our patch.
						// I.E. We patch with 15655155794400u, post-patch shows 15655155795m
						// Therefore, update the report item with the actual; post-patch memory.
						patchedContainer, _, _, foundPatchedContainer, err := util.FindContainerInUnstructured(patchedResource, reportItem.ResourceContainer)
						if !foundPatchedContainer || err != nil {
							glog.Errorf("unable to find the container %s in patched report item %s, and will have to set report memory limits to the value sent in the patch: %s - potential error = %v", reportItem.ResourceContainer, reportItem, newContainerMemoryLimits, err)
							reportItem.EndingMemory = newContainerMemoryLimits
						} else {
							patchedContainerMemoryLimits := patchedContainer.Resources.Limits.Memory()
							glog.V(1).Infof("setting report item %s EndingMemory to the post-patch value: %s", reportItem, patchedContainerMemoryLimits)
							reportItem.EndingMemory = patchedContainerMemoryLimits
						}
						glog.V(4).Infof("updating %s resourceVersion to %s after successful patch", reportItem, patchedResource.GetResourceVersion())
						reportItem.ResourceVersion = patchedResource.GetResourceVersion()
						glog.V(4).Infof("updating %s ResourceGeneration to %d after successful patch", reportItem, patchedResource.GetGeneration())
						reportItem.ResourceGeneration = patchedResource.GetGeneration()
					}
				}
				if !newContainerMemoryLimits.IsZero() && MLEquality > 0 {
					glog.V(1).Infof("%s memory limits will not be updated, the current %s limit cannot be increased by %.2f without exceeding the maximum allowed limit of %.1fX which is %s for this resource", reportItem, containerMemoryLimits, c.config.updateMemoryLimitsMultiplier, c.config.maxMemoryLimitsUpdateFactor, maxAllowedLimits)
				}
			} else {
				glog.V(1).Infof("%s memory limits will not be updated, %d OOM-kills has not yet reached the minimum threshold of %d", reportItem, reportItem.NumOOMs, c.config.updateMemoryMinimumOOMs)
			}
		} else {
			glog.V(1).Infof("not updating memory limits for %s as updating has not ben enabled", reportItem)
		}
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
