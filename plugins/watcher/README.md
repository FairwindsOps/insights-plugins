# Kubernetes Event Watcher

A Kubernetes plugin that watches policy-related resources and events, with special focus on policy violations that block resource installation.

## Features

- **Policy Event Watching**: Watches Kubernetes events and policy resources for policy violations
- **Policy Violation Detection**: Automatically detects and processes policy violations that block resource installation
- **Multi-format Support**: Handles both ValidatingAdmissionPolicy and regular Kyverno policy events
- **Insights Integration**: Sends blocked policy violations directly to Fairwinds Insights API
- **Real-time Processing**: Processes events as they occur in the cluster
- **Kyverno Integration**: Monitors Kyverno policy reports and cluster policy reports
- **Admission Control**: Tracks ValidatingAdmissionPolicy and MutatingAdmissionPolicy resources

## Usage

### Basic Usage

```bash
# Watch policy-related resources and events
./watcher

# Set log level
./watcher --log-level=debug
```

### Command Line Options

- `--log-level`: Log level - debug, info, warn, error (default: `info`)
- `--insights-host`: Fairwinds Insights hostname (optional)
- `--organization`: Fairwinds organization name (required if insights-host provided)
- `--cluster`: Cluster name (required if insights-host provided)
- `--insights-token`: Fairwinds Insights API token (required if insights-host provided)

### PolicyViolation Event Processing

The watcher provides special processing for PolicyViolation events:

- **Real-time Detection**: Captures `PolicyViolation` events as they occur
- **Blocked Policy Analysis**: Specifically identifies blocked policy violations that prevent resource installation
- **Insights API Integration**: Automatically sends any blocked policy violation to Fairwinds Insights
- **Multi-format Support**: Handles both ValidatingAdmissionPolicy and regular Kyverno policy events
- **Extensible Architecture**: Easy to add new event handlers for future requirements

#### Supported PolicyViolation Event Types

The watcher sends **any** PolicyViolation event that blocks resource installation:

- ✅ **ValidatingAdmissionPolicy events**: `validatingadmissionpolicy/policy-name` with `(blocked)`
- ✅ **Regular Kyverno policy events**: `deployment/nginx` with `policy namespace/policy-name fail (blocked): ...`
- ❌ **Non-blocked events**: Any violation without `(blocked)` in the message (warnings, audit violations)

#### Example PolicyViolation Events

```bash
# These events will be captured and sent to Insights API:
kubectl get events | grep PolicyViolation | grep "(blocked)"

# ValidatingAdmissionPolicy format:
Warning   PolicyViolation     validatingadmissionpolicy/disallow-host-path   Deployment default/nginx: [disallow-host-path] fail (blocked); HostPath volumes are forbidden...

# Regular Kyverno policy format:
Warning   PolicyViolation     deployment/nginx                               policy disallow-host-path/disallow-host-path fail (blocked): HostPath volumes are forbidden...
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

#### Usage with Insights API

```bash
# Run with Insights API integration
./watcher \
  --insights-host=https://insights.fairwinds.com \
  --organization=my-org \
  --cluster=production \
  --insights-token=your-api-token
```

### Extensible Event Handler System

The watcher uses a factory pattern for event handling, making it easy to add new event types:

- **PolicyViolation Events**: Captures any blocked policy violation (ValidatingAdmissionPolicy or Kyverno) and sends to Insights API
- **Kyverno Resources**: Handles PolicyReport, ClusterPolicyReport, Policy, and ClusterPolicy events
- **Generic Resources**: Fallback handler for any other Kubernetes resources
- **Easy Extension**: Add new handlers by implementing the `EventHandler` interface

#### Handler Architecture

```go
type EventHandler interface {
    Handle(watchedEvent *event.WatchedEvent) error
}
```

The factory automatically selects the most appropriate handler based on:
1. **Event characteristics** (e.g., `reason: PolicyViolation` → `policy-violation` handler)
2. **Resource type naming convention** (e.g., `PolicyReport` → `policyreport-handler`)
3. **Fallback to generic handler** for unmatched resources

**No `CanHandle` method needed** - the factory uses a simple naming convention!

#### Watched Resources
- **events** - Kubernetes events (CRITICAL for policy violations)
- **PolicyReport, ClusterPolicyReport** - Kyverno policy reports
- **Policy, ClusterPolicy** - Kyverno policies
- **ValidatingAdmissionPolicy, ValidatingAdmissionPolicyBinding** - Admission control policies
- **MutatingAdmissionPolicy, MutatingAdmissionPolicyBinding** - Admission control policies

## Building

```bash
go build -o watcher ./cmd/main.go
```

## Docker

```bash
docker build -t watcher .
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
# Kyverno policy resources
- apiGroups: ["wgpolicyk8s.io"]
  resources: ["policyreports", "clusterpolicyreports", "policies", "clusterpolicies"]
  verbs: ["get", "list", "watch"]
# ValidatingAdmissionPolicy resources
- apiGroups: ["admissionregistration.k8s.io"]
  resources: ["validatingadmissionpolicies", "validatingadmissionpolicybindings", "mutatingadmissionpolicies", "mutatingadmissionpolicybindings"]
  verbs: ["get", "list", "watch"]
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

