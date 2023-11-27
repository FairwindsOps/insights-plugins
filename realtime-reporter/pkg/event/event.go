package report

import (
	"strconv"
	"time"
)

const eventVersion = 1

type Event struct {
	Version   int            `json:"version"`
	Timestamp string         `json:"timestamp"`
	Namespace string         `json:"namespace"`
	Workload  string         `json:"workload"`
	Data      map[string]any `json:"data"`
}

func NewEvent(reportType string, namespace string, workload string, data map[string]any) Event {
	timestamp := strconv.FormatInt(time.Now().UTC().UnixNano(), 10)

	event := Event{
		Version:   eventVersion,
		Timestamp: timestamp,
		Namespace: namespace,
		Workload:  workload,
		Data:      data,
	}

	return event
}
