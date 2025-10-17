package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// EventSourceManager manages multiple event sources generically
type EventSourceManager struct {
	sources map[string]EventSource
	mu      sync.RWMutex
	wg      sync.WaitGroup
}

// NewEventSourceManager creates a new event source manager
func NewEventSourceManager() *EventSourceManager {
	return &EventSourceManager{
		sources: make(map[string]EventSource),
	}
}

// AddEventSource adds an event source to the manager
func (m *EventSourceManager) AddEventSource(name string, source EventSource) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sources[name] = source
	slog.Debug("Added event source", "name", name, "type", source.GetName())
}

// StartAll starts all enabled event sources
func (m *EventSourceManager) StartAll(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var errors []error

	for name, source := range m.sources {
		if !source.IsEnabled() {
			slog.Debug("Skipping disabled event source", "name", name)
			continue
		}

		m.wg.Add(1)
		go func(name string, source EventSource) {
			defer m.wg.Done()

			slog.Info("Starting event source", "name", name, "type", source.GetName())
			if err := source.Start(ctx); err != nil {
				slog.Error("Failed to start event source",
					"name", name,
					"type", source.GetName(),
					"error", err)
			}
		}(name, source)
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to start %d event sources", len(errors))
	}

	return nil
}

// StopAll stops all event sources
func (m *EventSourceManager) StopAll() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	slog.Info("Stopping all event sources")

	for name, source := range m.sources {
		slog.Debug("Stopping event source", "name", name, "type", source.GetName())
		source.Stop()
	}

	// Wait for all goroutines to finish
	m.wg.Wait()

	slog.Info("All event sources stopped")
}

// GetEventSourceCount returns the number of registered event sources
func (m *EventSourceManager) GetEventSourceCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.sources)
}

// GetEventSourceNames returns the names of all registered event sources
func (m *EventSourceManager) GetEventSourceNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.sources))
	for name := range m.sources {
		names = append(names, name)
	}

	return names
}
