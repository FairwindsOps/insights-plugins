package sync

import (
	"time"
)

// PolicySyncConfig represents the configuration for Kyverno policy sync
type PolicySyncConfig struct {
	DryRun           bool `mapstructure:"dryRun"`
	ValidatePolicies bool `mapstructure:"validatePolicies"`
}

// PolicySyncActions represents the actions to be taken during policy sync
type PolicySyncActions struct {
	ToApply  []string        `json:"toApply"`
	ToUpdate []string        `json:"toUpdate"`
	ToRemove []ClusterPolicy `json:"toRemove"`
}

// PolicySyncResult represents the result of a policy sync operation
type PolicySyncResult struct {
	Success  bool              `json:"success"`
	Actions  PolicySyncActions `json:"actions"`
	Applied  []string          `json:"applied"`
	Updated  []string          `json:"updated"`
	Removed  []string          `json:"removed"`
	Failed   []string          `json:"failed"`
	Errors   []string          `json:"errors"`
	Duration time.Duration     `json:"duration"`
	Summary  string            `json:"summary"`
}

// ClusterPolicy represents a Kyverno ClusterPolicy
type ClusterPolicy struct {
	Kind        string                 `json:"kind"`
	Name        string                 `json:"name"`
	Annotations map[string]string      `json:"annotations,omitempty"`
	Spec        map[string]interface{} `json:"spec,omitempty"`
	YAML        []byte                 `json:"yaml"`
}
