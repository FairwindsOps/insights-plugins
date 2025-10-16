# Kubernetes Event Watcher

A Kubernetes plugin that watches policy-related resources and events, with special focus on **ValidatingAdmissionPolicy violations** that block resource installation. Features **CloudWatch integration** for EKS clusters and **Automatic Policy Duplication** to create audit-only ValidatingAdmissionPolicies for capturing policy violations without blocking resources.

## Features

- **ValidatingAdmissionPolicy Focus**: Primary focus on ValidatingAdmissionPolicy violations from EKS CloudWatch logs
- **CloudWatch Integration**: Real-time processing of EKS audit logs from AWS CloudWatch
- **Dual Log Sources**: Supports both local audit logs (Kind/local) and CloudWatch logs (EKS)
- **Policy Violation Detection**: Automatically detects and processes policy violations that block resource installation
- **Multi-format Support**: Handles both ValidatingAdmissionPolicy and regular Kyverno policy events
- **Insights Integration**: Sends blocked policy violations directly to Fairwinds Insights API
- **Real-time Processing**: Processes events as they occur in the cluster (no historical data)
- **Performance Optimized**: Configurable batch sizes, memory limits, and CloudWatch filtering
- **IRSA Support**: Uses IAM Roles for Service Accounts for secure AWS access
- **Automatic Policy Duplication**: Automatically creates, updates, and deletes audit duplicates of ValidatingAdmissionPolicies with Deny-only actions
- **Audit Policy Support**: Supports dedicated audit policies for capturing policy violations without webhook dependencies

## Usage

### Basic Usage

#### Local Mode (Kind/Local Clusters)
```bash
# Watch policy-related resources and events from local audit logs
./insights-event-watcher --log-source=local

# Set log level
./insights-event-watcher --log-level=debug --log-source=local

# Run with Insights API integration
./insights-event-watcher \
  --log-source=local \
  --insights-host=https://insights.fairwinds.com \
  --organization=my-org \
  --cluster=production \
  --insights-token=your-api-token
```

#### CloudWatch Mode (EKS Clusters)
```bash
# Watch policy violations from EKS CloudWatch logs
./insights-event-watcher \
  --log-source=cloudwatch \
  --cloudwatch-log-group=/aws/eks/production-eks/cluster \
  --cloudwatch-region=us-west-2 \
  --cloudwatch-filter-pattern="{ $.stage = \"ResponseComplete\" && $.responseStatus.code >= 400 }" \
  --insights-host=https://insights.fairwinds.com \
  --organization=my-org \
  --cluster=production \
  --insights-token=your-api-token
```

### Command Line Options

#### General Options
- `--log-level`: Log level - debug, info, warn, error (default: `info`)
- `--insights-host`: Fairwinds Insights hostname (optional)
- `--organization`: Fairwinds organization name (required if insights-host provided)
- `--cluster`: Cluster name (required if insights-host provided)
- `--insights-token`: Fairwinds Insights API token (required if insights-host provided)
- `--event-buffer-size`: Size of the event processing buffer (default: `1000`)
- `--http-timeout-seconds`: HTTP client timeout in seconds (default: `30`)
- `--rate-limit-per-minute`: Maximum API calls per minute (default: `60`)

#### Log Source Options
- `--log-source`: Log source type - local, cloudwatch (default: `local`)

#### Local Mode Options
- `--audit-log-path`: Path to Kubernetes audit log file (optional)

#### CloudWatch Mode Options
- `--cloudwatch-log-group`: CloudWatch log group name (e.g., `/aws/eks/production-eks/cluster`)
- `--cloudwatch-region`: AWS region for CloudWatch logs
- `--cloudwatch-filter-pattern`: CloudWatch filter pattern for log events
- `--cloudwatch-batch-size`: Number of log events to process in each batch (default: `100`)
- `--cloudwatch-poll-interval`: Interval between CloudWatch log polls (default: `30s`)
- `--cloudwatch-max-memory`: Maximum memory usage in MB for CloudWatch processing (default: `512`)

## CloudWatch Integration

### Overview

The watcher supports real-time processing of EKS audit logs from AWS CloudWatch, enabling policy violation detection in production EKS clusters without requiring local audit log access.

### Key Features

- **Real-time Processing**: Processes only recent log events (last 5 minutes), no historical data
- **CloudWatch Filtering**: Uses CloudWatch filter patterns to reduce data transfer and processing
- **Performance Optimized**: Configurable batch sizes and memory limits for high-volume clusters
- **IRSA Authentication**: Uses IAM Roles for Service Accounts for secure AWS access
- **ValidatingAdmissionPolicy Focus**: Specifically designed to detect VAP violations from EKS audit logs

### IAM Setup

#### 1. Create IAM Role

Create an IAM role with the following trust policy for IRSA:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::ACCOUNT_ID:oidc-provider/OIDC_PROVIDER_URL"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "OIDC_PROVIDER_URL:sub": "system:serviceaccount:NAMESPACE:SERVICE_ACCOUNT_NAME"
        }
      }
    }
  ]
}
```

#### 2. Attach CloudWatch Policy

Attach the following policy to the IAM role:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "logs:DescribeLogGroups",
        "logs:DescribeLogStreams",
        "logs:FilterLogEvents",
        "logs:GetLogEvents"
      ],
      "Resource": [
        "arn:aws:logs:*:*:log-group:/aws/eks/*/cluster",
        "arn:aws:logs:*:*:log-group:/aws/eks/*/cluster:*"
      ]
    }
  ]
}
```

#### 3. Annotate Service Account

Add the IAM role annotation to the service account:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: insights-event-watcher
  annotations:
    eks.amazonaws.com/role-arn: "arn:aws:iam::ACCOUNT_ID:role/insights-watcher-cloudwatch-role"
```

### CloudWatch Filter Patterns

The watcher uses CloudWatch filter patterns to efficiently identify ValidatingAdmissionPolicy violations:

#### Basic Filter Pattern
```json
{ $.stage = "ResponseComplete" && $.responseStatus.code >= 400 && $.requestURI = "/api/v1/*" }
```

#### Advanced Filter Pattern
```json
{ $.stage = "ResponseComplete" && $.responseStatus.code >= 400 && $.requestURI = "/api/v1/*" && $.annotations."admission.k8s.io/validating-admission-policy" != null }
```

### Performance Tuning

#### For High-Volume Clusters
```bash
--cloudwatch-batch-size=500
--cloudwatch-poll-interval=15s
--cloudwatch-max-memory=1024
```

#### For Low-Volume Clusters
```bash
--cloudwatch-batch-size=50
--cloudwatch-poll-interval=60s
--cloudwatch-max-memory=256
```

### Troubleshooting CloudWatch

**Problem**: No CloudWatch logs being processed
- **Check**: Verify IAM role has correct permissions
- **Check**: Ensure log group name is correct (e.g., `/aws/eks/production-eks/cluster`)
- **Check**: Verify AWS region is correct
- **Check**: Look for CloudWatch client initialization errors in logs

**Problem**: High memory usage
- **Check**: Reduce `--cloudwatch-batch-size`
- **Check**: Increase `--cloudwatch-poll-interval`
- **Check**: Reduce `--cloudwatch-max-memory`

**Problem**: Missing policy violations
- **Check**: Verify filter pattern is correct
- **Check**: Ensure log group contains audit logs
- **Check**: Check for JSON parsing errors in logs

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

The watcher can be deployed as a Kubernetes deployment with support for both local and CloudWatch modes:

#### Local Mode Deployment (Kind/Local Clusters)
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
      serviceAccountName: insights-event-watcher
      containers:
      - name: watcher
        image: insights-event-watcher:latest
        command: ["/usr/local/bin/insights-event-watcher"]
        args:
        - "--log-source=local"
        - "--insights-host=https://insights.fairwinds.com"
        - "--organization=my-org"
        - "--cluster=production"
        - "--insights-token=your-api-token"
        - "--audit-log-path=/var/log/kubernetes/kube-apiserver-audit.log"
        volumeMounts:
        - name: audit-logs
          mountPath: /var/log/kubernetes
          readOnly: true
      volumes:
      - name: audit-logs
        hostPath:
          path: /var/log/kubernetes
          type: Directory
```

#### CloudWatch Mode Deployment (EKS Clusters)
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
      serviceAccountName: insights-event-watcher
      containers:
      - name: watcher
        image: insights-event-watcher:latest
        command: ["/usr/local/bin/insights-event-watcher"]
        args:
        - "--log-source=cloudwatch"
        - "--cloudwatch-log-group=/aws/eks/production-eks/cluster"
        - "--cloudwatch-region=us-west-2"
        - "--cloudwatch-filter-pattern={ $.stage = \"ResponseComplete\" && $.responseStatus.code >= 400 }"
        - "--cloudwatch-batch-size=100"
        - "--cloudwatch-poll-interval=30s"
        - "--cloudwatch-max-memory=512"
        - "--insights-host=https://insights.fairwinds.com"
        - "--organization=my-org"
        - "--cluster=production"
        - "--insights-token=your-api-token"
        env:
        - name: AWS_REGION
          value: "us-west-2"
        resources:
          requests:
            cpu: 100m
            memory: 256Mi
          limits:
            cpu: 500m
            memory: 1Gi
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: insights-event-watcher
  annotations:
    eks.amazonaws.com/role-arn: "arn:aws:iam::ACCOUNT_ID:role/insights-watcher-cloudwatch-role"
```

### Insights-Agent Helm Chart Integration

The watcher is integrated into the insights-agent Helm chart and can be configured via values.yaml:

#### Local Mode Configuration
```yaml
insights-event-watcher:
  enabled: true
  logLevel: "info"
  auditLogPath: "/var/log/kubernetes/kube-apiserver-audit.log"
  resources:
    limits:
      cpu: 100m
      memory: 128Mi
    requests:
      cpu: 50m
      memory: 64Mi
```

#### CloudWatch Mode Configuration
```yaml
insights-event-watcher:
  enabled: true
  logLevel: "info"
  cloudwatch:
    enabled: true
    logGroupName: "/aws/eks/production-eks/cluster"
    region: "us-west-2"
    filterPattern: "{ $.stage = \"ResponseComplete\" && $.responseStatus.code >= 400 && $.requestURI = \"/api/v1/*\" }"
    batchSize: 100
    pollInterval: "30s"
    maxMemoryMB: 512
  serviceAccount:
    annotations:
      eks.amazonaws.com/role-arn: "arn:aws:iam::ACCOUNT_ID:role/production-eks_cloudwatch_watcher"
  resources:
    limits:
      cpu: 500m
      memory: 1Gi
    requests:
      cpu: 100m
      memory: 256Mi
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
# ValidatingAdmissionPolicy resources - PRIMARY FOCUS
- apiGroups: ["admissionregistration.k8s.io"]
  resources: ["validatingadmissionpolicies", "validatingadmissionpolicybindings"]
  verbs: ["get", "list", "watch"]
# Kyverno policy resources (secondary)
- apiGroups: ["wgpolicyk8s.io"]
  resources: ["policyreports", "clusterpolicyreports"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["kyverno.io"]
  resources: ["policies", "clusterpolicies"]
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

### CloudWatch Issues

**Problem**: No CloudWatch logs being processed
- **Check**: Verify IAM role has correct permissions for CloudWatch Logs
- **Check**: Ensure log group name is correct (e.g., `/aws/eks/production-eks/cluster`)
- **Check**: Verify AWS region is correct
- **Check**: Look for CloudWatch client initialization errors in logs
- **Check**: Ensure service account has IRSA annotation

**Problem**: High memory usage with CloudWatch
- **Check**: Reduce `--cloudwatch-batch-size` (default: 100)
- **Check**: Increase `--cloudwatch-poll-interval` (default: 30s)
- **Check**: Reduce `--cloudwatch-max-memory` (default: 512MB)
- **Check**: Verify filter pattern is reducing log volume

**Problem**: Missing ValidatingAdmissionPolicy violations
- **Check**: Verify filter pattern includes VAP-specific conditions
- **Check**: Ensure log group contains EKS audit logs
- **Check**: Check for JSON parsing errors in logs
- **Check**: Verify VAP policies are actually blocking resources

