package data

import (
	"time"

	"github.com/falcosecurity/falcosidekick/types"
)

type OutputFormat struct {
	Output []FalcoOutput
}

// FalcoPayload is a struct to map falco event json
type FalcoOutput struct {
	PodName             string
	ControllerNamespace string
	ControllerName      string
	ControllerKind      string
	UUID                string                 `json:"uuid,omitempty"`
	Output              string                 `json:"output"`
	Priority            types.PriorityType     `json:"priority"`
	Rule                string                 `json:"rule"`
	Time                time.Time              `json:"time"`
	OutputFields        map[string]interface{} `json:"output_fields"`
}
