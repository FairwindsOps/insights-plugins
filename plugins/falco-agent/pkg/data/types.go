package data

import (
	"time"
)

type OutputFormat struct {
	Output []FalcoOutput
}

// FalcoPayload is a struct to map falco event json
type FalcoOutput struct {
	PodName             string
	Container           string
	ControllerNamespace string
	ControllerName      string
	ControllerKind      string
	UUID                string                 `json:"uuid,omitempty"`
	Output              string                 `json:"output"`
	Priority            int                    `json:"priority"`
	Rule                string                 `json:"rule"`
	Time                time.Time              `json:"time"`
	OutputFields        map[string]interface{} `json:"output_fields"`
}
