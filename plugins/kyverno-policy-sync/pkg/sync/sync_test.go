package sync

import (
	_ "embed"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

//go:embed testdata/single_policy.yaml
var singlePoliciesYAML string

//go:embed testdata/multiple_policies.yaml
var multiplePoliciesYAML string

// MockInsightsClient is a mock implementation of the insights client
type MockInsightsClient struct {
	mock.Mock
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
		ValidatePolicies: true,
	}
	assert.True(t, config.DryRun)
	assert.True(t, config.ValidatePolicies)
}

func TestPolicySyncActions(t *testing.T) {
	actions := PolicySyncActions{
		ToApply:  []string{"policy1", "policy2"},
		ToUpdate: []string{"policy3"},
		ToRemove: []ClusterPolicy{
			{Name: "policy4", Kind: "ClusterPolicy"},
		},
	}

	assert.Len(t, actions.ToApply, 2)
	assert.Len(t, actions.ToUpdate, 1)
	assert.Len(t, actions.ToRemove, 1)
	assert.Contains(t, actions.ToApply, "policy1")
	assert.Contains(t, actions.ToUpdate, "policy3")
	assert.Equal(t, "policy4", actions.ToRemove[0].Name)
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
		Spec: map[string]any{
			"validationFailureAction": "enforce",
		},
	}

	assert.Equal(t, "test-policy", policy.Name)
	assert.Equal(t, "Fairwinds Insights", policy.Annotations["insights.fairwinds.com/owned-by"])
	assert.Equal(t, "enforce", policy.Spec["validationFailureAction"])
}

func TestParseMultiplePoliciesFromYAML(t *testing.T) {
	policies, err := parsePoliciesFromYAML(singlePoliciesYAML)
	assert.NoError(t, err)
	assert.Len(t, policies, 1)

	policies, err = parsePoliciesFromYAML(multiplePoliciesYAML)
	assert.NoError(t, err)
	assert.Len(t, policies, 2)
}
