package report

import (
	"time"
)

const eventVersion = 1

type Event struct {
	EventVersion int            `json:"event_version"`
	Timestamp    int64          `json:"timestamp"`
	Kind         string         `json:"kind"`
	Namespace    string         `json:"namespace"`
	Workload     string         `json:"workload"`
	Data         map[string]any `json:"data"`
}

func NewEvent(kind string, namespace string, workload string, data map[string]any) Event {
	timestamp := time.Now().UTC().UnixNano()

	event := Event{
		EventVersion: eventVersion,
		Kind:         kind,
		Timestamp:    timestamp,
		Namespace:    namespace,
		Workload:     workload,
		Data:         data,
	}

	return event
}
