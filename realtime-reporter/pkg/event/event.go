package report

const eventVersion = 1

type Event struct {
	EventVersion int            `json:"event_version"`
	Timestamp    int64          `json:"timestamp"`
	Kind         string         `json:"kind"`
	Namespace    string         `json:"namespace"`
	Workload     string         `json:"workload"`
	Data         map[string]any `json:"data"`
}

func NewEvent(timestamp int64, kind string, namespace string, workload string, data map[string]any) Event {

	event := Event{
		EventVersion: eventVersion,
		Timestamp:    timestamp,
		Kind:         kind,
		Namespace:    namespace,
		Workload:     workload,
		Data:         data,
	}

	return event
}
