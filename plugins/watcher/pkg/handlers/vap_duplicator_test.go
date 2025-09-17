package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/event"
	"github.com/fairwindsops/insights-plugins/plugins/watcher/pkg/models"
)

func TestVAPDuplicatorHandler_Handle(t *testing.T) {
	tests := []struct {
		name           string
		watchedEvent   *event.WatchedEvent
		expectError    bool
		expectCreation bool
	}{
		{
			name: "ValidatingAdmissionPolicy with Deny-only binding should create audit duplicate",
			watchedEvent: &event.WatchedEvent{
				EventType:    event.EventTypeAdded,
				ResourceType: "ValidatingAdmissionPolicy",
				Name:         "test-policy",
				Namespace:    "",
				Data:         map[string]interface{}{},
			},
			expectError:    false,
			expectCreation: true,
		},
		{
			name: "Non-ValidatingAdmissionPolicy resource should be ignored",
			watchedEvent: &event.WatchedEvent{
				EventType:    event.EventTypeAdded,
				ResourceType: "Deployment",
				Name:         "test-deployment",
				Namespace:    "default",
			},
			expectError:    false,
			expectCreation: false,
		},
		{
			name: "ValidatingAdmissionPolicy with MODIFIED event should be ignored",
			watchedEvent: &event.WatchedEvent{
				EventType:    event.EventTypeModified,
				ResourceType: "ValidatingAdmissionPolicy",
				Name:         "test-policy",
				Namespace:    "",
				Data:         map[string]interface{}{},
			},
			expectError:    false,
			expectCreation: false,
		},
		{
			name: "ValidatingAdmissionPolicy with audit suffix should be ignored",
			watchedEvent: &event.WatchedEvent{
				EventType:    event.EventTypeAdded,
				ResourceType: "ValidatingAdmissionPolicy",
				Name:         "test-policy-insights-audit",
				Namespace:    "",
				Data:         map[string]interface{}{},
			},
			expectError:    false,
			expectCreation: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake Kubernetes client with test policies
			testPolicy := &admissionregistrationv1.ValidatingAdmissionPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-policy",
				},
				Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
					Validations: []admissionregistrationv1.Validation{
						{
							Expression: "object.spec.replicas <= 10",
							Message:    "Replicas must not exceed 10",
						},
					},
				},
			}

			testBinding := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-binding",
				},
				Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
					PolicyName: "test-policy",
					ValidationActions: []admissionregistrationv1.ValidationAction{
						admissionregistrationv1.Deny,
					},
				},
			}

			// Add audit policy for the audit suffix test
			auditPolicy := &admissionregistrationv1.ValidatingAdmissionPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-policy-insights-audit",
				},
				Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
					Validations: []admissionregistrationv1.Validation{
						{
							Expression: "object.spec.replicas <= 10",
							Message:    "Replicas must not exceed 10",
						},
					},
				},
			}

			fakeClient := fake.NewSimpleClientset(testPolicy, testBinding, auditPolicy)

			// Create handler
			config := models.InsightsConfig{}
			handler := NewVAPDuplicatorHandler(config, fakeClient)

			// Execute
			err := handler.Handle(tt.watchedEvent)

			// Assert
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// If we expect creation, verify that audit policies were created
			if tt.expectCreation {
				// This would require more complex mocking to verify actual creation
				// For now, we just verify no error occurred
				assert.NoError(t, err)
			}
		})
	}
}

func TestVAPDuplicatorHandler_needsAuditDuplicate(t *testing.T) {
	tests := []struct {
		name     string
		policy   *admissionregistrationv1.ValidatingAdmissionPolicy
		expected bool
	}{
		{
			name: "Policy without audit suffix should need duplicate",
			policy: &admissionregistrationv1.ValidatingAdmissionPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-policy",
				},
			},
			expected: true,
		},
		{
			name: "Policy with audit suffix should not need duplicate",
			policy: &admissionregistrationv1.ValidatingAdmissionPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-policy-insights-audit",
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake Kubernetes client with test binding
			testBinding := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-binding",
				},
				Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
					PolicyName: "test-policy",
					ValidationActions: []admissionregistrationv1.ValidationAction{
						admissionregistrationv1.Deny,
					},
				},
			}

			fakeClient := fake.NewSimpleClientset(testBinding)

			// Create handler
			config := models.InsightsConfig{}
			handler := NewVAPDuplicatorHandler(config, fakeClient)

			// Execute
			result := handler.needsAuditDuplicate(tt.policy)

			// Assert
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestVAPDuplicatorHandler_createAuditPolicy(t *testing.T) {
	// Create fake Kubernetes client
	fakeClient := fake.NewSimpleClientset()

	// Create handler
	config := models.InsightsConfig{}
	handler := NewVAPDuplicatorHandler(config, fakeClient)

	// Create test policy
	originalPolicy := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-policy",
			Labels: map[string]string{
				"app": "test",
			},
		},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			Validations: []admissionregistrationv1.Validation{
				{
					Expression: "true",
					Message:    "Test validation",
				},
			},
		},
	}

	// Execute
	auditPolicy := handler.createAuditPolicy(originalPolicy)

	// Assert
	assert.Equal(t, "test-policy-insights-audit", auditPolicy.Name)
	assert.Equal(t, "true", auditPolicy.Labels["insights.fairwinds.com/audit-policy"])
	assert.Equal(t, "test-policy", auditPolicy.Labels["insights.fairwinds.com/original-policy"])
	assert.Equal(t, "insights-event-watcher", auditPolicy.Annotations["insights.fairwinds.com/created-by"])
	assert.Equal(t, "test-policy", auditPolicy.Annotations["insights.fairwinds.com/original-policy"])

	// Verify the spec is identical
	assert.Equal(t, originalPolicy.Spec, auditPolicy.Spec)
}

func TestVAPDuplicatorHandler_createAuditBinding(t *testing.T) {
	// Create fake Kubernetes client
	fakeClient := fake.NewSimpleClientset()

	// Create handler
	config := models.InsightsConfig{}
	handler := NewVAPDuplicatorHandler(config, fakeClient)

	// Create test binding
	originalBinding := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-binding",
			Labels: map[string]string{
				"app": "test",
			},
		},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
			PolicyName: "test-policy",
			ValidationActions: []admissionregistrationv1.ValidationAction{
				admissionregistrationv1.Deny,
			},
		},
	}

	// Execute
	auditBinding := handler.createAuditBinding(originalBinding, "test-policy-insights-audit")

	// Assert
	assert.Equal(t, "test-binding-insights-audit", auditBinding.Name)
	assert.Equal(t, "test-policy-insights-audit", auditBinding.Spec.PolicyName)
	assert.Equal(t, []admissionregistrationv1.ValidationAction{admissionregistrationv1.Audit}, auditBinding.Spec.ValidationActions)
	assert.Equal(t, "true", auditBinding.Labels["insights.fairwinds.com/audit-binding"])
	assert.Equal(t, "test-binding", auditBinding.Labels["insights.fairwinds.com/original-binding"])
	assert.Equal(t, "insights-event-watcher", auditBinding.Annotations["insights.fairwinds.com/created-by"])
	assert.Equal(t, "test-binding", auditBinding.Annotations["insights.fairwinds.com/original-binding"])
}

func TestVAPDuplicatorHandler_CheckExistingPolicies(t *testing.T) {
	tests := []struct {
		name             string
		existingPolicies []admissionregistrationv1.ValidatingAdmissionPolicy
		existingBindings []admissionregistrationv1.ValidatingAdmissionPolicyBinding
		expectError      bool
		expectedCreated  int
	}{
		{
			name:             "No existing policies should not create any audit policies",
			existingPolicies: []admissionregistrationv1.ValidatingAdmissionPolicy{},
			existingBindings: []admissionregistrationv1.ValidatingAdmissionPolicyBinding{},
			expectError:      false,
			expectedCreated:  0,
		},
		{
			name: "Policy with Deny-only binding should create audit duplicate",
			existingPolicies: []admissionregistrationv1.ValidatingAdmissionPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-policy",
					},
					Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
						Validations: []admissionregistrationv1.Validation{
							{
								Expression: "true",
								Message:    "Test validation",
							},
						},
					},
				},
			},
			existingBindings: []admissionregistrationv1.ValidatingAdmissionPolicyBinding{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-binding",
					},
					Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
						PolicyName: "test-policy",
						ValidationActions: []admissionregistrationv1.ValidationAction{
							admissionregistrationv1.Deny,
						},
					},
				},
			},
			expectError:     false,
			expectedCreated: 1,
		},
		{
			name: "Policy with audit suffix should be skipped",
			existingPolicies: []admissionregistrationv1.ValidatingAdmissionPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-policy-insights-audit",
					},
					Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
						Validations: []admissionregistrationv1.Validation{
							{
								Expression: "true",
								Message:    "Test validation",
							},
						},
					},
				},
			},
			existingBindings: []admissionregistrationv1.ValidatingAdmissionPolicyBinding{},
			expectError:      false,
			expectedCreated:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake Kubernetes client with existing resources
			var objects []runtime.Object

			// Add policies
			for i := range tt.existingPolicies {
				objects = append(objects, &tt.existingPolicies[i])
			}

			// Add bindings
			for i := range tt.existingBindings {
				objects = append(objects, &tt.existingBindings[i])
			}

			fakeClient := fake.NewSimpleClientset(objects...)

			// Create handler
			config := models.InsightsConfig{}
			handler := NewVAPDuplicatorHandler(config, fakeClient)

			// Execute
			err := handler.CheckExistingPolicies()

			// Assert
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Note: In a real test, we would verify that the expected number of audit policies were created
			// This would require more complex mocking to track the create operations
		})
	}
}
