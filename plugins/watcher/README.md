# Kubernetes Event Watcher

A Kubernetes plugin that watches policy-related resources and events, with special focus on policy violations that block resource installation. Features **Automatic Policy Duplication** to create audit-only ValidatingAdmissionPolicies for capturing policy violations without blocking resources.

## Features

- **Policy Event Watching**: Watches Kubernetes events and policy resources for policy violations
- **Policy Violation Detection**: Automatically detects and processes policy violations that block resource installation
- **Multi-format Support**: Handles both ValidatingAdmissionPolicy and regular Kyverno policy events
- **Insights Integration**: Sends blocked policy violations directly to Fairwinds Insights API
- **Real-time Processing**: Processes events as they occur in the cluster
- **Kyverno Integration**: Monitors Kyverno policy reports and cluster policy reports
- **Admission Control**: Tracks ValidatingAdmissionPolicy and ValidatingAdmissionPolicyBinding resources
- **Automatic Policy Duplication**: Automatically creates, updates, and deletes audit duplicates of ValidatingAdmissionPolicies with Deny-only actions
- **Audit Policy Support**: Supports dedicated audit policies for capturing policy violations without webhook dependencies

## Usage

### Basic Usage

```bash
# Watch policy-related resources and events
./insights-event-watcher

# Set log level
./insights-event-watcher --log-level=debug

# Run with Insights API integration
./insights-event-watcher \
  --insights-host=https://insights.fairwinds.com \
  --organization=my-org \
  --cluster=production \
  --insights-token=your-api-token
```

### Command Line Options

- `--log-level`: Log level - debug, info, warn, error (default: `info`)
- `--insights-host`: Fairwinds Insights hostname (optional)
- `--organization`: Fairwinds organization name (required if insights-host provided)
- `--cluster`: Cluster name (required if insights-host provided)
- `--insights-token`: Fairwinds Insights API token (required if insights-host provided)

### Automatic Policy Duplication

The watcher plugin now automatically creates audit duplicates of ValidatingAdmissionPolicies that have only "Deny" actions. This feature:

- **Automatic Detection**: Monitors for new ValidatingAdmissionPolicy resources
- **Smart Duplication**: Only creates audit duplicates for policies with Deny-only bindings
- **Audit Policy Creation**: Creates `{policy-name}-insights-audit` policies with Audit-only actions
- **Audit Binding Creation**: Creates `{binding-name}-insights-audit` bindings pointing to audit policies
- **Duplicate Prevention**: Skips policies that already have audit duplicates

#### How It Works

1. **Startup Check**: On startup, checks all existing ValidatingAdmissionPolicy resources
2. **Policy Detection**: Watches for new, modified, and deleted ValidatingAdmissionPolicy resources
3. **Binding Analysis**: Checks if any bindings have only "Deny" actions
4. **Audit Creation**: Creates audit policy and bindings if needed
5. **Audit Updates**: Updates audit policies when original policies are modified
6. **Audit Cleanup**: Deletes audit policies when original policies are deleted
7. **Event Generation**: Audit policies generate events for the watcher to capture

#### Example

When you create a policy like this:
```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: disallow-host-path
spec:
  validations:
  - expression: "!has(object.spec.volumes) || !object.spec.volumes.exists(v, has(v.hostPath))"
    message: "HostPath volumes are forbidden"
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicyBinding
metadata:
  name: disallow-host-path-binding
spec:
  policyName: disallow-host-path
  validationActions:
  - Deny  # Only Deny action
```

The watcher automatically creates:
```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: disallow-host-path-insights-audit
  labels:
    insights.fairwinds.com/audit-policy: "true"
    insights.fairwinds.com/original-policy: "disallow-host-path"
spec:
  validations:
  - expression: "!has(object.spec.volumes) || !object.spec.volumes.exists(v, has(v.hostPath))"
    message: "HostPath volumes are forbidden"
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicyBinding
metadata:
  name: disallow-host-path-binding-insights-audit
  labels:
    insights.fairwinds.com/audit-binding: "true"
    insights.fairwinds.com/original-binding: "disallow-host-path-binding"
spec:
  policyName: disallow-host-path-insights-audit
  validationActions:
  - Audit  # Only Audit action
```

### Manual Audit Policy Approach

You can also manually create audit policies instead of relying on automatic duplication. This approach provides:

- **Clean Separation**: Dedicated audit policies separate from enforcement policies
- **Explicit Configuration**: Clear intent with Audit-only validation actions
- **Better Performance**: No automatic duplication overhead

See [AUDIT_POLICY_SETUP.md](examples/AUDIT_POLICY_SETUP.md) for detailed manual setup instructions.


### PolicyViolation Event Processing

The watcher provides special processing for PolicyViolation events:

- **Real-time Detection**: Captures `PolicyViolation` and `VAPViolation` events as they occur
- **Blocked Policy Analysis**: Specifically identifies blocked policy violations that prevent resource installation
- **Insights API Integration**: Automatically sends any blocked policy violation to Fairwinds Insights
- **Multi-format Support**: Handles both ValidatingAdmissionPolicy and regular Kyverno policy events
- **Extensible Architecture**: Easy to add new event handlers for future requirements

#### Supported PolicyViolation Event Types

The watcher sends **any** PolicyViolation event that blocks resource installation:

- ✅ **ValidatingAdmissionPolicy events**: `validatingadmissionpolicy/policy-name` with `(blocked)`
- ✅ **Regular Kyverno policy events**: `deployment/nginx` with `policy namespace/policy-name fail (blocked): ...`
- ✅ **VAPViolation events**: `VAPViolation` events from audit policies
- ❌ **Non-blocked events**: Any violation without `(blocked)` in the message (warnings, audit violations)

#### Example PolicyViolation Events

```bash
# These events will be captured and sent to Insights API:
kubectl get events | grep -E "PolicyViolation|VAPViolation" | grep "(blocked)"

# ValidatingAdmissionPolicy format:
Warning   PolicyViolation     validatingadmissionpolicy/disallow-host-path   Deployment default/nginx: [disallow-host-path] fail (blocked); HostPath volumes are forbidden...

# Regular Kyverno policy format:
Warning   PolicyViolation     deployment/nginx                               policy disallow-host-path/disallow-host-path fail (blocked): HostPath volumes are forbidden...

# VAPViolation format (from audit policies):
Warning   VAPViolation        replicaset/nginx-578557b98b                   VAP Policy Violation: ReplicaSet default/nginx-578557b98b: [disallow-host-path] fail; HostPath volumes are forbidden...
```

#### Creating Policies That Generate Blocked Events

To generate events that will be sent to Insights, create policies with blocking behavior:

**Kyverno Policy Example:**
```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: disallow-host-path
spec:
  validationFailureAction: Enforce  # This generates "(blocked)" events
  rules:
  - name: disallow-host-path
    match:
      resources:
        kinds: [Pod]
    validate:
      message: "HostPath volumes are forbidden"
      pattern:
        spec:
          =(volumes):
            - X(hostPath): "null"
```

**ValidatingAdmissionPolicy Example:**
```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingAdmissionPolicy
metadata:
  name: disallow-host-path
spec:
  failurePolicy: Fail  # This generates "(blocked)" events
  validations:
  - expression: "!has(object.spec.volumes) || !object.spec.volumes.exists(v, has(v.hostPath))"
    message: "HostPath volumes are forbidden"
  matchConstraints:
    resourceRules:
    - apiGroups: [""]
      apiVersions: ["v1"]
      operations: ["CREATE", "UPDATE"]
      resources: ["pods"]
```


### Extensible Event Handler System

The watcher uses a factory pattern for event handling, making it easy to add new event types:

- **PolicyViolation Events**: Captures any blocked policy violation (ValidatingAdmissionPolicy or Kyverno) and sends to Insights API
- **VAPViolation Events**: Processes VAPViolation events from audit policies
- **ValidatingAdmissionPolicy Events**: Handles ValidatingAdmissionPolicy resources for automatic audit policy duplication
- **Easy Extension**: Add new handlers by implementing the `EventHandler` interface

#### Handler Architecture

```go
type EventHandler interface {
    Handle(watchedEvent *event.WatchedEvent) error
}
```

The factory automatically selects the most appropriate handler based on:
1. **Event characteristics** (e.g., `reason: PolicyViolation` or `reason: VAPViolation` → `policy-violation` handler)
2. **Resource type** (e.g., `ValidatingAdmissionPolicy` → `vap-duplicator` handler)
3. **Fallback to no handler** for unmatched resources

**No `CanHandle` method needed** - the factory uses a simple naming convention!

#### Watched Resources
- **events** - Kubernetes events (CRITICAL for policy violations)
- **ValidatingAdmissionPolicy, ValidatingAdmissionPolicyBinding** - Admission control policies for automatic audit policy duplication

## Building

```bash
# Build for local development
go build -o insights-event-watcher ./cmd/insights-event-watcher/main.go

# Build for Linux (for Docker)
GOOS=linux GOARCH=amd64 go build -o insights-event-watcher ./cmd/insights-event-watcher/main.go
```

## Docker

```bash
# Build Docker image
docker build -t insights-event-watcher .

# Run the watcher
docker run insights-event-watcher
```

## Deployment

### Kubernetes Deployment

The watcher can be deployed as a Kubernetes deployment:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: insights-event-watcher
  namespace: insights-agent
spec:
  replicas: 1
  selector:
    matchLabels:
      app: insights-event-watcher
  template:
    metadata:
      labels:
        app: insights-event-watcher
    spec:
      containers:
      - name: watcher
        image: insights-event-watcher:latest
        command: ["/usr/local/bin/insights-event-watcher"]
        args:
        - "--insights-host=https://insights.fairwinds.com"
        - "--organization=my-org"
        - "--cluster=production"
        - "--insights-token=your-api-token"
```

### Testing Automatic Policy Duplication

To test the automatic policy duplication functionality:

1. **Deploy the watcher** with proper RBAC permissions
2. **Create a ValidatingAdmissionPolicy** with Deny-only bindings
3. **Check for automatically created audit policies**:

```bash
# Check for audit policies
kubectl get validatingadmissionpolicies | grep insights-audit

# Check watcher logs for VAP duplicator activity
kubectl logs -n insights-agent deployment/insights-event-watcher | grep -i "VAPDuplicator"
```

## Configuration

The watcher uses in-cluster configuration by default. Ensure it has appropriate RBAC permissions to watch the desired resources.

### Required RBAC Permissions

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: watcher
rules:
# Kubernetes events - CRITICAL for policy violation detection
- apiGroups: [""]
  resources: ["events"]
  verbs: ["get", "list", "watch"]
# ValidatingAdmissionPolicy resources
- apiGroups: ["admissionregistration.k8s.io"]
  resources: ["validatingadmissionpolicies", "validatingadmissionpolicybindings"]
  verbs: ["get", "list", "watch", "create"]  # Added "create" for automatic audit policy duplication
```

## Event Types

- `ADDED`: Resource was created
- `MODIFIED`: Resource was updated
- `DELETED`: Resource was deleted
- `ERROR`: Error occurred while watching

## Logging

The watcher provides structured logging with the following fields:
- `event_type`: Type of Kubernetes event
- `resource_type`: Type of resource
- `namespace`: Resource namespace
- `name`: Resource name
- `uid`: Resource UID
- `timestamp`: Event timestamp

## Troubleshooting


### Automatic Policy Duplication Issues

**Problem**: Audit policies are not being created automatically
- **Check**: Verify RBAC permissions include `create` on `validatingadmissionpolicies` and `validatingadmissionpolicybindings`
- **Check**: Look for VAP duplicator handler logs: `kubectl logs deployment/insights-event-watcher | grep "VAPDuplicator"`
- **Check**: Look for startup check logs: `kubectl logs deployment/insights-event-watcher | grep "Checking existing ValidatingAdmissionPolicies"`
- **Check**: Ensure the policy has bindings with only "Deny" actions
- **Check**: Verify the policy name doesn't already end with "-insights-audit"

**Problem**: Duplicate audit policies being created
- **Check**: The duplicator includes duplicate prevention logic
- **Check**: Look for "Audit policy already exists" messages in logs
- **Check**: Verify existing audit policies have proper labels

**Problem**: Audit policies created but no events generated
- **Check**: Verify audit bindings have "Audit" validation actions
- **Check**: Ensure audit policies have the same validation rules as original policies
- **Check**: Test with a violating resource to trigger policy evaluation

### General Issues

**Problem**: No policy violations being sent to Insights
- **Check**: Ensure events contain `(blocked)` or `fail:` in the message
- **Check**: Verify Insights API credentials are correct
- **Check**: Look for "Sending blocked policy violation to Insights" log messages

**Problem**: Watcher not detecting events
- **Check**: Verify RBAC permissions include `watch` on `events` resource
- **Check**: Ensure the watcher is running in the correct namespace
- **Check**: Look for "No handler found for event" debug messages

