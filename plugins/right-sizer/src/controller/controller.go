package controller

import (
	"time"

	"github.com/fairwindsops/insights-plugins/right-sizer/src/report"
	"github.com/fairwindsops/insights-plugins/right-sizer/src/util"
	"github.com/golang/glog"
	core "k8s.io/api/core/v1"

	"github.com/google/go-cmp/cmp"
	"github.com/thoas/go-funk"
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

// Controller is a controller that listens for Pod OOM-kill events,
// optionally changes memory limits of the
// pod-controller, and manages action-items in an Insights report.
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
	updateMemoryLimits            bool
	updateMemoryLimitsIncrement   float64 // Multiplied by current limits
	updateMemoryLimitsMax         float64 // Multiplied by first-seen limits.
	updateMemoryLimitsMinimumOOMs int64
	allowedNamespaces             []string // Limits all operations (alert or update).
	allowedUpdateNamespaces       []string // Limits updating memory limits.
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

// WithAllowedUpdateNamespaces sets the corresponding field in a controllerConfig type.
func WithAllowedUpdateNamespaces(value []string) controllerOption {
	return func(c *controllerConfig) {
		c.allowedUpdateNamespaces = value
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

// WithUpdateMemoryLimitsIncrement sets the corresponding field in a controllerConfig type.
func WithUpdateMemoryLimitsIncrement(value float64) controllerOption {
	return func(c *controllerConfig) {
		if value > 0.0 {
			c.updateMemoryLimitsIncrement = value
		}
		return
	}
}

// WithUpdateMemoryLimitsMax sets the corresponding field in a
// controllerConfig type.
func WithUpdateMemoryLimitsMax(value float64) controllerOption {
	return func(c *controllerConfig) {
		if value > 0.0 {
			c.updateMemoryLimitsMax = value
		}
		return
	}
}

// WithUpdateMemoryLimitsMinimumOOMs sets the corresponding field in a controllerConfig type.
func WithUpdateMemoryLimitsMinimumOOMs(value int64) controllerOption {
	return func(c *controllerConfig) {
		if value > 0 {
			c.updateMemoryLimitsMinimumOOMs = value
		}
		return
	}
}

// NewController returns an instance of the Controller, setting configuration
// defaults unless optional configuration is specified by calling `Withxxx()`
// functions as parameters to this constructor.
func NewController(stop chan struct{}, kubeClientResources util.KubeClientResources, rb *report.RightSizerReportBuilder, options ...controllerOption) *Controller {
	cfg := &controllerConfig{
		updateMemoryLimits:            false,
		updateMemoryLimitsMinimumOOMs: 2,
		updateMemoryLimitsIncrement:   1.2,
		updateMemoryLimitsMax:         2.0,
	}

	// Process functional options, overriding the above defaults.
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
	glog.Infof("Controller configuration is: %+v", c.config)
	glog.Infof("Report configuration is: %s", c.reportBuilder.GetConfigAsString())
	dissimilarNamespaces := funk.LeftJoinString(c.config.allowedUpdateNamespaces, c.config.allowedNamespaces)
	if c.config.updateMemoryLimits && len(dissimilarNamespaces) > 0 {
		glog.Errorf("NOTE: memory limits will not be updated in these Kubernetes namespaces: %v Although these namespaces are in the allowedUpdateNamespaces list, they are missing from the global allowedNamespaces one.", dissimilarNamespaces)
	}
	err := c.reportBuilder.ReadConfigMap()
	if err != nil {
		glog.Errorf("while attempting to read state from ConfigMap: %v", err)
	}
	go c.reportBuilder.LoopRemoveOldItems()
	c.reportBuilder.RunServer()
	c.k8sFactory.Start(c.stopCh)
	c.k8sFactory.WaitForCacheSync(c.Stop)

	for {
		select {
		case event := <-c.eventAddedCh:
			c.processEvent(event)
		case eventUpdate := <-c.eventUpdatedCh:
			c.processEventUpdate(eventUpdate)
		case <-c.Stop:
			glog.Info("Stopping")
			return nil
		}
	}
}

const startedEvent = "Started"
const podKind = "Pod"

// isContainerStartedEvent returns true if the Kubernetes event is a container
// having been started.
func isContainerStartedEvent(event *core.Event) bool {
	return (event.Reason == startedEvent &&
		event.InvolvedObject.Kind == podKind)
}

// isSameEventOccurrence returns true if the Kubernetes event is a duplicate.
func isSameEventOccurrence(g *eventUpdateGroup) bool {
	return (g.oldEvent.InvolvedObject == g.newEvent.InvolvedObject &&
		g.oldEvent.Count == g.newEvent.Count)
}

// processEvent determines whether a Kubernetes event relates to an OOM-killed
// pod, or an update related to an existing report item.
func (c *Controller) processEvent(event *core.Event) {
	glog.V(4).Infof("got event %s/%s (count: %d), reason: %s, involved object: %s", event.ObjectMeta.Namespace, event.ObjectMeta.Name, event.Count, event.Reason, event.InvolvedObject.Kind)
	if len(c.config.allowedNamespaces) > 0 && !funk.ContainsString(c.config.allowedNamespaces, event.ObjectMeta.Namespace) {
		glog.V(4).Infof("ignoring event %s/%s as its namespace is not allowed", event.ObjectMeta.Namespace, event.ObjectMeta.Name)
		return
	}
	if isContainerStartedEvent(event) {
		pod, err := c.podLister.Pods(event.InvolvedObject.Namespace).Get(event.InvolvedObject.Name)
		if err != nil {
			glog.Errorf("Failed to retrieve pod %s/%s, due to: %v", event.InvolvedObject.Namespace, event.InvolvedObject.Name, err)
			return
		}
		c.processOOMKilledPod(pod)
		return
	}
	if event.InvolvedObject.Kind == podKind {
		glog.V(6).Infof("not processing this event for a potential pod-controller update, because it is for a pod: %s/%s (count: %d), reason: %s, involved object: %s", event.ObjectMeta.Namespace, event.ObjectMeta.Name, event.Count, event.Reason, event.InvolvedObject.Kind)
		return
	}
	// This event is not an OOM-kill, but may relate to an update that changes
	// memory limits of an existing report item. I.E. a ReplicaSet scaling a
	// deployment that has been updated.
	err := c.reportBuilder.RemoveRelatedItemsOlderWithModifiedMemoryLimits(event.InvolvedObject)
	if err != nil {
		glog.Errorf("%v", err)
	}
}

// processEventUpdate is a wrapper for processEvent(), which first verifies
// an event is not a duplicate.
func (c *Controller) processEventUpdate(eventUpdate *eventUpdateGroup) {
	event := eventUpdate.newEvent
	if eventUpdate.oldEvent == nil {
		glog.V(4).Infof("No old event present for event %s/%s (count: %d), reason: %s, involved object: %s, skipping processing", event.ObjectMeta.Namespace, event.ObjectMeta.Name, event.Count, event.Reason, event.InvolvedObject.Kind)
		return
	}
	if cmp.Equal(eventUpdate.oldEvent, eventUpdate.newEvent) {
		glog.V(4).Infof("Event %s/%s (count: %d), reason: %s, involved object: %s, did not change: skipping processing", event.ObjectMeta.Namespace, event.ObjectMeta.Name, event.Count, event.Reason, event.InvolvedObject.Kind)
		return
	}
	if isSameEventOccurrence(eventUpdate) {
		glog.V(3).Infof("Event %s/%s (count: %d), reason: %s, involved object: %s, did not change wrt. to restart count: skipping processing", eventUpdate.newEvent.ObjectMeta.Namespace, eventUpdate.newEvent.ObjectMeta.Name, eventUpdate.newEvent.Count, eventUpdate.newEvent.Reason, eventUpdate.newEvent.InvolvedObject.Kind)
		return
	}
	c.processEvent(event)
}

// processOOMKilledPod takes action on a pod that has a OOM-killed container,
// including updating metrics and optionally updating memory limits of the
// owning pod-controller.
func (c *Controller) processOOMKilledPod(pod *core.Pod) {
	for _, s := range pod.Status.ContainerStatuses {
		// The first OOM-kill of a pod is reflected in .State.Terminated instead
		// of .LastTerminationState.Terminated.
		if (s.LastTerminationState.Terminated == nil || s.LastTerminationState.Terminated.Reason != TerminationReasonOOMKilled) && (s.State.Terminated == nil || s.State.Terminated.Reason != TerminationReasonOOMKilled) {
			ProcessedContainerUpdates.WithLabelValues("not_oomkilled").Inc()
			continue
		}

		if (s.LastTerminationState.Terminated != nil && s.LastTerminationState.Terminated.FinishedAt.Time.Before(c.startTime)) || (s.State.Terminated != nil && s.State.Terminated.FinishedAt.Time.Before(c.startTime)) {
			glog.V(1).Infof("The container '%s' in '%s/%s' was terminated before this controller started - termination-time=%s, controller-start-time=%s", s.Name, pod.Namespace, pod.Name, s.LastTerminationState.Terminated.FinishedAt.Time, c.startTime)
			ProcessedContainerUpdates.WithLabelValues("oomkilled_termination_too_old").Inc()
			continue
		}

		c.recorder.Eventf(pod, core.EventTypeWarning, "PreviousContainerWasOOMKilled", "The previous instance of the container '%s' (%s) was OOMKilled", s.Name, s.ContainerID)
		ProcessedContainerUpdates.WithLabelValues("oomkilled_event_sent").Inc()

		containerInfo, _, foundContainer := util.FindContainerInPodSpec(&pod.Spec, s.Name)
		if !foundContainer {
			glog.Errorf("cannot find OOM-killed container %s in pod %v", s.Name, pod)
			continue
		}
		podControllerObject, err := util.GetControllerFromPod(c.kubeClientResources, pod)
		if err != nil {
			glog.Errorf("unable to get owning controller for pod %s/%s: %v", pod.Namespace, pod.Name, err)
			return
		}
		glog.V(1).Infof("Pod %s/%s is owned by pod-controller %s %s", pod.Namespace, pod.Name, podControllerObject.GetKind(), podControllerObject.GetName())
		reportItem := c.reportBuilder.CreateRightSizerReportItem(podControllerObject, containerInfo)

		if c.config.updateMemoryLimits {
			if len(c.config.allowedUpdateNamespaces) > 0 && funk.ContainsString(c.config.allowedUpdateNamespaces, pod.Namespace) {
				if c.config.updateMemoryLimitsMinimumOOMs > 0 && reportItem.NumOOMs >= c.config.updateMemoryLimitsMinimumOOMs {
					containerMemoryLimits := containerInfo.Resources.Limits.Memory()
					newContainerMemoryLimits := reportItem.IncrementMemory(containerMemoryLimits, c.config.updateMemoryLimitsIncrement, c.config.updateMemoryLimitsMax)
					if !newContainerMemoryLimits.IsZero() {
						glog.V(1).Infof("%s has memory  limit %v, updating to %v", reportItem, containerMemoryLimits, newContainerMemoryLimits)
						patchedResource, err := util.PatchContainerMemoryLimits(c.kubeClientResources, podControllerObject, reportItem.ResourceContainer, newContainerMemoryLimits)
						if err != nil {
							// EndingMemory remains the previously set StartingMemory value.
							glog.Errorf("error patching %s memory limits: %v", reportItem, err)
						} else {
							// The post-patch memory limits may be different than the limits in our patch.
							// I.E. We patch with 15655155794400u, post-patch shows 15655155795m
							// Update  report item with the actual; post-patch memory.
							patchedContainer, _, _, foundPatchedContainer, err := util.FindContainerInUnstructured(patchedResource, reportItem.ResourceContainer)
							if !foundPatchedContainer || err != nil {
								glog.Errorf("unable to find the container %s in patched report item %s, and will have to set report memory limits to the value sent in the patch: %s - potential error = %v", reportItem.ResourceContainer, reportItem, newContainerMemoryLimits, err)
								reportItem.EndingMemory = newContainerMemoryLimits
							} else {
								patchedContainerMemoryLimits := patchedContainer.Resources.Limits.Memory()
								glog.V(1).Infof("setting report item %s EndingMemory to the post-patch value: %s", reportItem, patchedContainerMemoryLimits)
								reportItem.EndingMemory = patchedContainerMemoryLimits
							}
							glog.V(4).Infof("updating %s ResourceGeneration to %d after successful patch", reportItem, patchedResource.GetGeneration())
							reportItem.ResourceGeneration = patchedResource.GetGeneration()
						}
					}
				} else {
					glog.V(1).Infof("%s memory limits will not be updated, %d OOM-kills has not yet reached the minimum threshold of %d", reportItem, reportItem.NumOOMs, c.config.updateMemoryLimitsMinimumOOMs)
				}
			} else {
				glog.V(1).Infof("%s memory limits will not be updated, namespace %s is not allowed via updateMemoryLimitNamespace", reportItem, pod.Namespace)
			}
		} else {
			glog.V(1).Infof("not updating memory limits for %s as updating has not ben enabled", reportItem)
		}
		c.reportBuilder.StoreItem(*reportItem)
		err = c.reportBuilder.WriteConfigMap()
		if err != nil {
			glog.Error(err)
		}
	}
}
