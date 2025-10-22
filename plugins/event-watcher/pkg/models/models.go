package models

type InsightsConfig struct {
	Hostname     string
	Organization string
	Cluster      string
	Token        string
}

type CloudWatchConfig struct {
	LogGroupName  string
	Region        string
	FilterPattern string
	BatchSize     int
	PollInterval  string
	MaxMemoryMB   int
}
type EventReport struct {
	EventType    string                 `json:"eventType"`
	ResourceType string                 `json:"resourceType"`
	Namespace    string                 `json:"namespace"`
	Name         string                 `json:"name"`
	UID          string                 `json:"uid"`
	Timestamp    int64                  `json:"timestamp"`
	Data         map[string]interface{} `json:"data"`
	Metadata     map[string]interface{} `json:"metadata"`
}

type PolicyViolationEvent struct {
	EventReport
	Policies     map[string]map[string]string `json:"policies"`
	PolicyResult string                       `json:"policyResult"`
	Message      string                       `json:"message"`
	Blocked      bool                         `json:"blocked"`
	EventTime    string                       `json:"eventTime,omitempty"` // Kubernetes eventTime
}

type EventHandlerConfig struct {
	Enabled     bool              `json:"enabled"`
	EventTypes  []string          `json:"event_types"`
	Filters     map[string]string `json:"filters"`
	InsightsAPI bool              `json:"insights_api"`
}

type WatcherConfig struct {
	Insights      InsightsConfig                `json:"insights"`
	EventHandlers map[string]EventHandlerConfig `json:"event_handlers"`
	KyvernoOnly   bool                          `json:"kyverno_only"`
	LogLevel      string                        `json:"log_level"`
}
