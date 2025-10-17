package event

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNewWatchedEvent(t *testing.T) {
	tests := []struct {
		name          string
		eventType     EventType
		obj           interface{}
		resourceType  string
		expectError   bool
		errorContains string
	}{
		{
			name:      "valid unstructured object",
			eventType: EventTypeAdded,
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Event",
					"metadata": map[string]interface{}{
						"name":      "test-event",
						"namespace": "default",
						"uid":       "test-uid-123",
					},
					"eventTime": "2024-01-15T10:30:00Z",
				},
			},
			resourceType: "events",
			expectError:  false,
		},
		{
			name:      "object without metadata",
			eventType: EventTypeAdded,
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Event",
				},
			},
			resourceType:  "events",
			expectError:   true,
			errorContains: "object missing metadata field",
		},
		{
			name:      "object with invalid metadata type",
			eventType: EventTypeAdded,
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Event",
					"metadata":   "invalid-metadata",
				},
			},
			resourceType:  "events",
			expectError:   true,
			errorContains: "metadata is not a map",
		},
		{
			name:          "invalid object type",
			eventType:     EventTypeAdded,
			obj:           "invalid-object",
			resourceType:  "events",
			expectError:   true,
			errorContains: "unable to convert object to unstructured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := NewWatchedEvent(tt.eventType, tt.obj, tt.resourceType)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				assert.Nil(t, event)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, event)
				assert.Equal(t, tt.eventType, event.EventType)
				assert.Equal(t, tt.resourceType, event.ResourceType)
				assert.Equal(t, EventVersion, event.EventVersion)
				assert.NotZero(t, event.Timestamp)
			}
		})
	}
}

func TestWatchedEventIsKyvernoResource(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		expected     bool
	}{
		{
			name:         "PolicyReport is Kyverno resource",
			resourceType: "PolicyReport",
			expected:     true,
		},
		{
			name:         "ClusterPolicyReport is Kyverno resource",
			resourceType: "ClusterPolicyReport",
			expected:     true,
		},
		{
			name:         "Policy is Kyverno resource",
			resourceType: "Policy",
			expected:     true,
		},
		{
			name:         "ClusterPolicy is Kyverno resource",
			resourceType: "ClusterPolicy",
			expected:     true,
		},
		{
			name:         "ValidatingAdmissionPolicy is Kyverno resource",
			resourceType: "ValidatingAdmissionPolicy",
			expected:     true,
		},
		{
			name:         "ValidatingAdmissionPolicyBinding is Kyverno resource",
			resourceType: "ValidatingAdmissionPolicyBinding",
			expected:     true,
		},
		{
			name:         "MutatingAdmissionPolicy is Kyverno resource",
			resourceType: "MutatingAdmissionPolicy",
			expected:     true,
		},
		{
			name:         "MutatingAdmissionPolicyBinding is Kyverno resource",
			resourceType: "MutatingAdmissionPolicyBinding",
			expected:     true,
		},
		{
			name:         "events is not Kyverno resource",
			resourceType: "events",
			expected:     false,
		},
		{
			name:         "pods is not Kyverno resource",
			resourceType: "pods",
			expected:     false,
		},
		{
			name:         "empty resource type is not Kyverno resource",
			resourceType: "",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &WatchedEvent{
				ResourceType: tt.resourceType,
			}
			assert.Equal(t, tt.expected, event.IsKyvernoResource())
		})
	}
}

func TestWatchedEventGetPolicyName(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		eventName    string
		data         map[string]interface{}
		expected     string
	}{
		{
			name:         "Policy resource returns name",
			resourceType: "Policy",
			eventName:    "test-policy",
			expected:     "test-policy",
		},
		{
			name:         "ClusterPolicy resource returns name",
			resourceType: "ClusterPolicy",
			eventName:    "test-cluster-policy",
			expected:     "test-cluster-policy",
		},
		{
			name:         "PolicyReport with results returns policy name",
			resourceType: "PolicyReport",
			eventName:    "test-report",
			data: map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{
						"policy": "test-policy",
					},
				},
			},
			expected: "test-policy",
		},
		{
			name:         "ClusterPolicyReport with results returns policy name",
			resourceType: "ClusterPolicyReport",
			eventName:    "test-cluster-report",
			data: map[string]interface{}{
				"results": []interface{}{
					map[string]interface{}{
						"policy": "test-cluster-policy",
					},
				},
			},
			expected: "test-cluster-policy",
		},
		{
			name:         "PolicyReport without results returns name",
			resourceType: "PolicyReport",
			eventName:    "test-report",
			data:         map[string]interface{}{},
			expected:     "test-report",
		},
		{
			name:         "non-Kyverno resource returns empty",
			resourceType: "events",
			eventName:    "test-event",
			expected:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &WatchedEvent{
				ResourceType: tt.resourceType,
				Name:         tt.eventName,
				Data:         tt.data,
			}
			assert.Equal(t, tt.expected, event.GetPolicyName())
		})
	}
}

func TestWatchedEventComplexData(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "wgpolicyk8s.io/v1alpha2",
			"kind":       "PolicyReport",
			"metadata": map[string]interface{}{
				"name":      "test-report",
				"namespace": "default",
				"uid":       "test-uid-456",
			},
			"results": []interface{}{
				map[string]interface{}{
					"policy":  "test-policy",
					"result":  "fail",
					"message": "Policy violation detected",
				},
			},
		},
	}

	event, err := NewWatchedEvent(EventTypeModified, obj, "PolicyReport")
	require.NoError(t, err)

	assert.Equal(t, "test-report", event.Name)
	assert.Equal(t, "default", event.Namespace)
	assert.Equal(t, "test-uid-456", event.UID)
	assert.Equal(t, "test-policy", event.GetPolicyName())

	// Verify complex data is preserved
	results, ok := event.Data["results"].([]interface{})
	require.True(t, ok)
	require.Len(t, results, 1)

	result, ok := results[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "test-policy", result["policy"])
	assert.Equal(t, "fail", result["result"])
}
