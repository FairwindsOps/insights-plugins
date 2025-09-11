package models

type InsightsConfig struct {
	Hostname     string
	Organization string
	Cluster      string
	Token        string
}
type EventReport struct {
	EventType    string                 `json:"event_type"`
	ResourceType string                 `json:"resource_type"`
	Namespace    string                 `json:"namespace"`
	Name         string                 `json:"name"`
	UID          string                 `json:"uid"`
	Timestamp    int64                  `json:"timestamp"`
	Data         map[string]interface{} `json:"data"`
	Metadata     map[string]interface{} `json:"metadata"`
}

type PolicyViolationEvent struct {
	EventReport
	PolicyName   string `json:"policy_name"`
	PolicyResult string `json:"policy_result"`
	Message      string `json:"message"`
	Blocked      bool   `json:"blocked"`
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
	OutputDir     string                        `json:"output_dir"`
	KyvernoOnly   bool                          `json:"kyverno_only"`
	LogLevel      string                        `json:"log_level"`
}
