package watcher

import (
	"context"
)

// EventSource represents a generic source of events that can be monitored
type EventSource interface {
	// Start begins monitoring the event source
	Start(ctx context.Context) error

	// Stop stops monitoring the event source
	Stop()

	// GetName returns a human-readable name for this event source
	GetName() string

	// IsEnabled returns whether this event source is currently enabled
	IsEnabled() bool
}
