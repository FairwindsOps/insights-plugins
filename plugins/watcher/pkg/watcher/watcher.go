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
	wg             sync.WaitGroup
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
	handlerFactory := handlers.NewEventHandlerFactory(insightsConfig, kubeClient.KubeInterface)

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

	// Check existing ValidatingAdmissionPolicies for audit duplicates
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		if err := w.checkExistingPolicies(); err != nil {
			logrus.WithError(err).Error("Failed to check existing policies for audit duplicates")
		}
	}()

	// Start event processor
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.processEvents()
	}()

	// Start informers
	for _, informer := range w.informers {
		w.wg.Add(1)
		go func(informer cache.SharedInformer) {
			defer w.wg.Done()
			informer.Run(w.stopCh)
		}(informer)
	}

	logrus.Info("Kubernetes watcher started successfully")
	return nil
}

// Stop stops the watcher
func (w *Watcher) Stop() {
	logrus.Info("Stopping Kubernetes watcher")
	close(w.stopCh)

	// Wait for all goroutines to finish
	w.wg.Wait()

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
	// Watch resources needed for policy violation detection
	return []string{
		// Kubernetes events - CRITICAL for policy violations (contains PolicyViolation events)
		"events",
		// Kyverno policy resources
		"PolicyReport",
		"ClusterPolicyReport",
		"Policy",
		"ClusterPolicy",
		// ValidatingAdmissionPolicy resources
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
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.watchEvents(resourceType, watcher)
	}()

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
		// Event successfully queued
	default:
		logrus.WithFields(logrus.Fields{
			"resource_type": resourceType,
			"event_type":    eventType,
			"namespace":     watchedEvent.Namespace,
			"name":          watchedEvent.Name,
		}).Warn("Event channel full, dropping event - consider increasing buffer size")
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

			if err := w.handlerFactory.ProcessEvent(watchedEvent); err != nil {
				logrus.WithError(err).Error("failed to process event through handlers")
			}
		}
	}
}

// checkExistingPolicies checks existing ValidatingAdmissionPolicies for audit duplicates
func (w *Watcher) checkExistingPolicies() error {
	// Get the VAP duplicator handler from the factory
	handler := w.handlerFactory.GetHandler(&event.WatchedEvent{
		ResourceType: "ValidatingAdmissionPolicy",
	})
	
	if handler == nil {
		logrus.Debug("No VAP duplicator handler found, skipping existing policy check")
		return nil
	}

	// Type assert to VAPDuplicatorHandler
	vapDuplicator, ok := handler.(*handlers.VAPDuplicatorHandler)
	if !ok {
		logrus.Debug("Handler is not VAPDuplicatorHandler, skipping existing policy check")
		return nil
	}

	// Call the CheckExistingPolicies method
	return vapDuplicator.CheckExistingPolicies()
}
