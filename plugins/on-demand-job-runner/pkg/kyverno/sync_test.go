package kyverno

import (
	"testing"
	"time"

	"github.com/FairwindsOps/insights-plugins/on-demand-job-runner/pkg/insights"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockInsightsClient is a mock implementation of the insights client
type MockInsightsClient struct {
	mock.Mock
}

func (m *MockInsightsClient) ClaimOnDemandJobs(limit int) ([]insights.OnDemandJob, error) {
	args := m.Called(limit)
	return args.Get(0).([]insights.OnDemandJob), args.Error(1)
}

func (m *MockInsightsClient) UpdateOnDemandJobStatus(jobID int64, status insights.OnDemandJobStatus) error {
	args := m.Called(jobID, status)
	return args.Error(0)
}

func (m *MockInsightsClient) GetClusterKyvernoPoliciesYAML() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

// MockK8sClient is a mock implementation of the Kubernetes client
type MockK8sClient struct {
	mock.Mock
}

func (m *MockK8sClient) GetClusterKyvernoPoliciesYAML() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func TestPolicySyncConfig(t *testing.T) {
	config := PolicySyncConfig{
		DryRun:           true,
		SyncInterval:     15 * time.Minute,
		LockTimeout:      30 * time.Minute,
		ValidatePolicies: true,
	}

	assert.True(t, config.DryRun)
	assert.Equal(t, 15*time.Minute, config.SyncInterval)
	assert.Equal(t, 30*time.Minute, config.LockTimeout)
	assert.True(t, config.ValidatePolicies)
}

func TestPolicySyncActions(t *testing.T) {
	actions := PolicySyncActions{
		ToApply:  []string{"policy1", "policy2"},
		ToUpdate: []string{"policy3"},
		ToRemove: []string{"policy4"},
	}

	assert.Len(t, actions.ToApply, 2)
	assert.Len(t, actions.ToUpdate, 1)
	assert.Len(t, actions.ToRemove, 1)
	assert.Contains(t, actions.ToApply, "policy1")
	assert.Contains(t, actions.ToUpdate, "policy3")
	assert.Contains(t, actions.ToRemove, "policy4")
}

func TestPolicySyncResult(t *testing.T) {
	result := PolicySyncResult{
		Success:  true,
		Actions:  PolicySyncActions{ToApply: []string{"policy1"}},
		Applied:  []string{"policy1"},
		Updated:  []string{},
		Removed:  []string{},
		Failed:   []string{},
		Errors:   []string{},
		Duration: 5 * time.Second,
		DryRun:   false,
		Summary:  "Policy sync completed: Applied 1, Updated 0, Removed 0, Failed 0, Duration: 5s",
	}

	assert.True(t, result.Success)
	assert.Len(t, result.Applied, 1)
	assert.Empty(t, result.Errors)
	assert.Equal(t, 5*time.Second, result.Duration)
	assert.False(t, result.DryRun)
}

func TestClusterPolicy(t *testing.T) {
	policy := ClusterPolicy{
		Name: "test-policy",
		Annotations: map[string]string{
			"insights.fairwinds.com/owned-by": "Fairwinds Insights",
		},
		Spec: map[string]interface{}{
			"validationFailureAction": "enforce",
		},
	}

	assert.Equal(t, "test-policy", policy.Name)
	assert.Equal(t, "Fairwinds Insights", policy.Annotations["insights.fairwinds.com/owned-by"])
	assert.Equal(t, "enforce", policy.Spec["validationFailureAction"])
}

func TestPolicySyncLock(t *testing.T) {
	lock := PolicySyncLock{
		FilePath:    "/tmp/test.lock",
		LockTimeout: 30 * time.Minute,
	}

	assert.Equal(t, "/tmp/test.lock", lock.FilePath)
	assert.Equal(t, 30*time.Minute, lock.LockTimeout)
}

func TestValidationResult(t *testing.T) {
	result := ValidationResult{
		Valid:    true,
		Errors:   []string{},
		Warnings: []string{"This is a warning"},
	}

	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
	assert.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Warnings, "This is a warning")
}
