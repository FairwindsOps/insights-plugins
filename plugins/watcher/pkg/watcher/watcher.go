package watcher

import (
	"context"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/client"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/event"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/handlers"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/models"
)

// Watcher manages watching Kubernetes resources
type Watcher struct {
	client         *client.Client
	watchers       map[string]watch.Interface
	informers      map[string]cache.SharedInformer
	eventChannel   chan *event.WatchedEvent
	stopCh         chan struct{}
	mu             sync.RWMutex
	handlerFactory *handlers.EventHandlerFactory
	insightsConfig models.InsightsConfig
}

// NewWatcher creates a new Kubernetes watcher
func NewWatcher(insightsConfig models.InsightsConfig) (*Watcher, error) {
	kubeClient, err := client.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Create handler factory
	handlerFactory := handlers.NewEventHandlerFactory(insightsConfig)

	w := &Watcher{
		client:         kubeClient,
		watchers:       make(map[string]watch.Interface),
		informers:      make(map[string]cache.SharedInformer),
		eventChannel:   make(chan *event.WatchedEvent, 1000),
		stopCh:         make(chan struct{}),
		handlerFactory: handlerFactory,
		insightsConfig: insightsConfig,
	}

	return w, nil
}

// Start begins watching Kubernetes resources
func (w *Watcher) Start(ctx context.Context) error {
	logrus.Info("Starting Kubernetes watcher")

	// Define resources to watch
	resources := w.getResourcesToWatch()

	// Start watching each resource type
	for _, resourceType := range resources {
		if err := w.watchResource(ctx, resourceType); err != nil {
			logrus.WithError(err).WithField("resource", resourceType).Warn("Failed to start watching resource")
			continue
		}
		logrus.WithField("resource", resourceType).Info("Started watching resource")
	}

	// Start event processor
	go w.processEvents()

	// Start informers
	for _, informer := range w.informers {
		go informer.Run(w.stopCh)
	}

	logrus.Info("Kubernetes watcher started successfully")
	return nil
}

// Stop stops the watcher
func (w *Watcher) Stop() {
	logrus.Info("Stopping Kubernetes watcher")
	close(w.stopCh)

	w.mu.Lock()
	defer w.mu.Unlock()

	// Stop all watchers
	for resourceType, watcher := range w.watchers {
		watcher.Stop()
		logrus.WithField("resource", resourceType).Info("Stopped watching resource")
	}

	close(w.eventChannel)
	logrus.Info("Kubernetes watcher stopped")
}

// getResourcesToWatch returns the list of resources to watch
func (w *Watcher) getResourcesToWatch() []string {

	// Watch all common Kubernetes resources
	return []string{
		// Core resources
		"pods",
		"services",
		"deployments",
		"replicasets",
		"statefulsets",
		"daemonsets",
		"jobs",
		"cronjobs",
		"configmaps",
		"secrets",
		"persistentvolumes",
		"persistentvolumeclaims",
		"nodes",
		"namespaces",
		"events", // This is crucial for PolicyViolation events

		// RBAC
		"roles",
		"clusterroles",
		"rolebindings",
		"clusterrolebindings",
		"serviceaccounts",

		// Network
		"ingresses",
		"networkpolicies",

		// Storage
		"storageclasses",

		// Kyverno resources
		"PolicyReport",
		"ClusterPolicyReport",
		"Policy",
		"ClusterPolicy",
		"ValidatingAdmissionPolicy",
		"ValidatingAdmissionPolicyBinding",
		"MutatingAdmissionPolicy",
		"MutatingAdmissionPolicyBinding",
	}
}

func (w *Watcher) watchResource(ctx context.Context, resourceType string) error {
	resourceInterface, err := w.client.WatchResources(ctx, resourceType)
	if err != nil {
		return fmt.Errorf("failed to get resource interface for %s: %w", resourceType, err)
	}

	watcher, err := resourceInterface.Watch(ctx, v1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to start watcher for %s: %w", resourceType, err)
	}

	w.mu.Lock()
	w.watchers[resourceType] = watcher
	w.mu.Unlock()

	// Start watching for events
	go w.watchEvents(resourceType, watcher)

	return nil
}

func (w *Watcher) watchEvents(resourceType string, watcher watch.Interface) {
	defer watcher.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case event, ok := <-watcher.ResultChan():
			if !ok {
				logrus.WithField("resource", resourceType).Warn("Watcher channel closed")
				return
			}

			if err := w.handleEvent(resourceType, event); err != nil {
				logrus.WithError(err).WithField("resource", resourceType).Error("Failed to handle event")
			}
		}
	}
}

func (w *Watcher) handleEvent(resourceType string, kubeEvent watch.Event) error {
	var eventType event.EventType

	switch kubeEvent.Type {
	case watch.Added:
		eventType = event.EventTypeAdded
	case watch.Modified:
		eventType = event.EventTypeModified
	case watch.Deleted:
		eventType = event.EventTypeDeleted
	case watch.Error:
		eventType = event.EventTypeError
	default:
		return fmt.Errorf("unknown event type: %s", kubeEvent.Type)
	}

	watchedEvent, err := event.NewWatchedEvent(eventType, kubeEvent.Object, resourceType)
	if err != nil {
		return fmt.Errorf("failed to create watched event: %w", err)
	}

	select {
	case w.eventChannel <- watchedEvent:
	default:
		logrus.Warn("Event channel full, dropping event")
	}

	return nil
}

func (w *Watcher) processEvents() {
	for {
		select {
		case <-w.stopCh:
			return
		case watchedEvent, ok := <-w.eventChannel:
			if !ok {
				return
			}

			watchedEvent.LogEvent()

			// Process event through factory (no more if statements!)
			if err := w.handlerFactory.ProcessEvent(watchedEvent); err != nil {
				logrus.WithError(err).Error("failed to process event through handlers")
			}
		}
	}
}
