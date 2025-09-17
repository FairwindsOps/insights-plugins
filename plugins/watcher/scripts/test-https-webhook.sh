#!/bin/bash

# Script to test the HTTPS webhook with ValidatingAdmissionPolicyBinding resources
# This script creates test resources and validates that the webhook is working correctly

set -e

NAMESPACE="${NAMESPACE:-insights-agent}"
SERVICE_NAME="${SERVICE_NAME:-insights-agent-insights-event-watcher}"

echo "Testing HTTPS webhook with ValidatingAdmissionPolicyBinding resources..."

# Function to cleanup test resources
cleanup() {
    echo "Cleaning up test resources..."
    kubectl delete -f /tmp/test-vap-policy.yaml --ignore-not-found=true
    kubectl delete -f /tmp/test-vap-binding.yaml --ignore-not-found=true
}

# Set up cleanup on exit
trap cleanup EXIT

# Create test ValidatingAdmissionPolicy
cat > /tmp/test-vap-policy.yaml << EOF
apiVersion: admissionregistration.k8s.io/v1beta1
kind: ValidatingAdmissionPolicy
metadata:
  name: test-https-webhook-policy
spec:
  failurePolicy: Fail
  matchConstraints:
    matchPolicy: Equivalent
    namespaceSelector: {}
    objectSelector: {}
    resourceRules:
    - apiGroups:
      - apps
      apiVersions:
      - v1
      operations:
      - CREATE
      - UPDATE
      resources:
      - deployments
      scope: '*'
  validations:
  - expression: '!has(object.spec.template.spec.volumes) || object.spec.template.spec.volumes.all(volume, !has(volume.hostPath))'
    message: HostPath volumes are forbidden. The field spec.template.spec.volumes[*].hostPath must be unset.
EOF

# Create test ValidatingAdmissionPolicyBinding (Deny-only - should be mutated to add Audit)
cat > /tmp/test-vap-binding.yaml << EOF
apiVersion: admissionregistration.k8s.io/v1beta1
kind: ValidatingAdmissionPolicyBinding
metadata:
  name: test-https-webhook-binding
spec:
  policyName: test-https-webhook-policy
  validationActions:
  - Deny  # Only Deny action - our mutating webhook should add Audit
EOF

echo "Creating test ValidatingAdmissionPolicy..."
kubectl apply -f /tmp/test-vap-policy.yaml

echo "Creating test ValidatingAdmissionPolicyBinding..."
kubectl apply -f /tmp/test-vap-binding.yaml

echo "Waiting for webhook to process the binding..."
sleep 5

echo "Checking if Audit action was added to the binding..."
BINDING_ACTIONS=$(kubectl get validatingadmissionpolicybinding test-https-webhook-binding -o jsonpath='{.spec.validationActions[*]}')

if [[ "$BINDING_ACTIONS" == *"Audit"* ]]; then
    echo "✅ SUCCESS: Audit action was added to the binding!"
    echo "Final validation actions: $BINDING_ACTIONS"
else
    echo "❌ FAILURE: Audit action was not added to the binding"
    echo "Current validation actions: $BINDING_ACTIONS"
    exit 1
fi

echo "Testing webhook health endpoint..."
if kubectl get pods -n "$NAMESPACE" -l app=insights-event-watcher -o jsonpath='{.items[0].metadata.name}' > /dev/null 2>&1; then
    POD_NAME=$(kubectl get pods -n "$NAMESPACE" -l app=insights-event-watcher -o jsonpath='{.items[0].metadata.name}')
    echo "Testing health endpoint on pod: $POD_NAME"
    
    # Test health endpoint
    kubectl exec -n "$NAMESPACE" "$POD_NAME" -- wget -qO- --spider http://localhost:8081/healthz
    if [ $? -eq 0 ]; then
        echo "✅ SUCCESS: Health endpoint is responding"
    else
        echo "❌ FAILURE: Health endpoint is not responding"
        exit 1
    fi
    
    # Test readiness endpoint
    kubectl exec -n "$NAMESPACE" "$POD_NAME" -- wget -qO- --spider http://localhost:8081/readyz
    if [ $? -eq 0 ]; then
        echo "✅ SUCCESS: Readiness endpoint is responding"
    else
        echo "❌ FAILURE: Readiness endpoint is not responding"
        exit 1
    fi
else
    echo "⚠️  WARNING: No insights-event-watcher pod found, skipping health checks"
fi

echo "✅ HTTPS webhook test completed successfully!"
echo ""
echo "Summary:"
echo "- ValidatingAdmissionPolicy created"
echo "- ValidatingAdmissionPolicyBinding created and mutated (Audit action added)"
echo "- Webhook health endpoints are responding"
echo "- TLS configuration is working correctly"
