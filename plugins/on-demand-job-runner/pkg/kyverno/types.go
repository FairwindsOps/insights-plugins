package kyverno

import (
	"time"
)

// PolicySyncConfig represents the configuration for Kyverno policy sync
type PolicySyncConfig struct {
	DryRun           bool          `mapstructure:"dryRun"`
	SyncInterval     time.Duration `mapstructure:"syncInterval"`
	LockTimeout      time.Duration `mapstructure:"lockTimeout"`
	ValidatePolicies bool          `mapstructure:"validatePolicies"`
}

// PolicySyncActions represents the actions to be taken during policy sync
type PolicySyncActions struct {
	ToApply  []string `json:"toApply"`
	ToUpdate []string `json:"toUpdate"`
	ToRemove []string `json:"toRemove"`
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
	DryRun   bool              `json:"dryRun"`
	Summary  string            `json:"summary"`
}

// ClusterPolicy represents a Kyverno ClusterPolicy
type ClusterPolicy struct {
	Name        string                 `json:"name"`
	Annotations map[string]string      `json:"annotations,omitempty"`
	Spec        map[string]interface{} `json:"spec,omitempty"`
}

// PolicySyncLock represents a file-based lock for preventing concurrent sync operations
type PolicySyncLock struct {
	FilePath    string
	LockTimeout time.Duration
}

// ValidationResult represents the result of policy validation
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}
