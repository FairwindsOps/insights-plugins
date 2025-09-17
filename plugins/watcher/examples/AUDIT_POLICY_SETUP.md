# Audit Policy Setup for Insights Event Watcher

This document describes how to set up ValidatingAdmissionPolicy resources with Audit actions for the Insights Event Watcher plugin to capture policy violations.

## Overview

Instead of using a mutating webhook to add Audit actions to existing policies, you can create duplicate ValidatingAdmissionPolicy resources with Audit-only actions. This approach provides:

1. **Clean Separation**: Dedicated audit policies separate from enforcement policies
2. **No Webhook Dependency**: Doesn't require mutating webhook functionality
3. **Explicit Configuration**: Clear intent with Audit-only validation actions
4. **Better Performance**: No webhook overhead for policy creation

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Kubernetes Cluster                          │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐    ┌─────────────────┐                    │
│  │ Enforcement VAP │    │   Audit VAP     │                    │
│  │ (Deny action)   │    │ (Audit action)  │                    │
│  └─────────────────┘    └─────────────────┘                    │
│           │                       │                            │
│           ▼                       ▼                            │
│  ┌─────────────────┐    ┌─────────────────┐                    │
│  │ Blocks Resource │    │ Generates Event │                    │
│  │ Creation        │    │ for Watcher     │                    │
│  └─────────────────┘    └─────────────────┘                    │
│           │                       │                            │
│           ▼                       ▼                            │
│  ┌─────────────────┐    ┌─────────────────┐                    │
│  │ Resource        │    │ Insights Event  │                    │
│  │ Rejected        │    │ Watcher         │                    │
│  └─────────────────┘    └─────────────────┘                    │
└─────────────────────────────────────────────────────────────────┘
```

## Setup Instructions

### 1. Create Audit Policy

Create a ValidatingAdmissionPolicy with the same validation rules as your enforcement policy:

```yaml
apiVersion: admissionregistration.k8s.io/v1beta1
kind: ValidatingAdmissionPolicy
metadata:
  name: test-deny-only-insights-audit
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
```

### 2. Create Audit Policy Binding

Create a ValidatingAdmissionPolicyBinding with Audit-only actions:

```yaml
apiVersion: admissionregistration.k8s.io/v1beta1
kind: ValidatingAdmissionPolicyBinding
metadata:
  name: test-deny-only-binding-insights-audit
spec:
  policyName: test-deny-only-insights-audit
  validationActions:
  - Audit  # Only Audit action - for watcher plugin to capture violations
```

### 3. Deploy the Policies

```bash
# Apply the audit policy and binding
kubectl apply -f vap-audit-policy.yaml
```

## Policy Naming Convention

To maintain clarity between enforcement and audit policies, use this naming convention:

- **Enforcement Policy**: `{policy-name}` (e.g., `disallow-host-path`)
- **Audit Policy**: `{policy-name}-insights-audit` (e.g., `disallow-host-path-insights-audit`)
- **Enforcement Binding**: `{policy-name}-binding` (e.g., `disallow-host-path-binding`)
- **Audit Binding**: `{binding-name}-insights-audit` (e.g., `disallow-host-path-binding-insights-audit`)

## Example: Complete Setup

Here's a complete example with both enforcement and audit policies:

### Enforcement Policy (Blocks Resources)

```yaml
apiVersion: admissionregistration.k8s.io/v1beta1
kind: ValidatingAdmissionPolicy
metadata:
  name: disallow-host-path
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
---
apiVersion: admissionregistration.k8s.io/v1beta1
kind: ValidatingAdmissionPolicyBinding
metadata:
  name: disallow-host-path-binding
spec:
  policyName: disallow-host-path
  validationActions:
  - Deny  # Only Deny action - blocks resource creation
```

### Audit Policy (Generates Events)

```yaml
apiVersion: admissionregistration.k8s.io/v1beta1
kind: ValidatingAdmissionPolicy
metadata:
  name: disallow-host-path-insights-audit
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
---
apiVersion: admissionregistration.k8s.io/v1beta1
kind: ValidatingAdmissionPolicyBinding
metadata:
  name: disallow-host-path-binding-insights-audit
spec:
  policyName: disallow-host-path-insights-audit
  validationActions:
  - Audit  # Only Audit action - generates events for watcher
```

## Testing the Setup

### 1. Deploy Both Policies

```bash
# Deploy enforcement policy
kubectl apply -f enforcement-policy.yaml

# Deploy audit policy
kubectl apply -f audit-policy.yaml
```

### 2. Test Policy Violation

Create a deployment that violates the policy:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-hostpath-deployment
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-hostpath
  template:
    metadata:
      labels:
        app: test-hostpath
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
        volumeMounts:
        - name: hostpath-volume
          mountPath: /hostpath
      volumes:
      - name: hostpath-volume
        hostPath:
          path: /tmp  # This violates the policy
```

### 3. Verify Results

```bash
# Check that the deployment was blocked (enforcement policy)
kubectl get deployment test-hostpath-deployment
# Should show no deployment or failed status

# Check for audit events (audit policy)
kubectl get events --field-selector reason=PolicyViolation
# Should show policy violation events

# Check watcher logs
kubectl logs -n insights-agent -l app=insights-event-watcher | grep "PolicyViolation"
```

## Benefits of Audit Policy Approach

### Advantages

1. **No Webhook Dependency**: Doesn't require mutating webhook functionality
2. **Clear Separation**: Explicit separation between enforcement and audit
3. **Better Performance**: No webhook overhead for policy creation
4. **Easier Debugging**: Clear distinction between blocking and audit policies
5. **Independent Scaling**: Audit policies can be scaled independently

### Considerations

1. **Policy Duplication**: Need to maintain two copies of each policy
2. **Synchronization**: Changes to enforcement policies need to be replicated to audit policies
3. **Resource Usage**: Slightly more Kubernetes resources for duplicate policies

## Migration from Webhook Approach

If you're currently using the mutating webhook approach and want to migrate to audit policies:

1. **Create audit policies** for existing enforcement policies
2. **Deploy audit policies** alongside existing enforcement policies
3. **Verify event generation** from audit policies
4. **Remove mutating webhook** configuration (optional)
5. **Clean up webhook resources** (optional)

## Troubleshooting

### Common Issues

1. **No audit events generated**:
   - Verify audit policy binding has `Audit` action
   - Check that audit policy has same validation rules as enforcement policy
   - Ensure watcher is running and has proper RBAC permissions

2. **Duplicate events**:
   - Make sure enforcement and audit policies have different names
   - Check that only audit policy binding has `Audit` action

3. **Policy not matching resources**:
   - Verify `matchConstraints` are identical between enforcement and audit policies
   - Check `resourceRules` and `operations` match your test resources

### Verification Commands

```bash
# Check policy bindings
kubectl get validatingadmissionpolicybinding -o wide

# Check validation actions
kubectl get validatingadmissionpolicybinding -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.validationActions[*]}{"\n"}{end}'

# Check for policy violation events
kubectl get events --field-selector reason=PolicyViolation --sort-by='.lastTimestamp'

# Check watcher logs
kubectl logs -n insights-agent -l app=insights-event-watcher | grep -i "policy"
```

## Files Reference

- `vap-audit-policy.yaml`: Complete audit policy and binding example
- `test-vap-deny-only.yaml`: Enforcement policy example
- `AUDIT_POLICY_SETUP.md`: This documentation file
