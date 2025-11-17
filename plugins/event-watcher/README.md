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
- **Backpressure Handling**: Intelligent retry logic when event channel is full, preventing event loss
- **Metrics & Monitoring**: Built-in metrics for dropped events, processing rates, and channel utilization
- **Health Check Endpoints**: HTTP endpoints for Kubernetes liveness and readiness probes
- **Graceful Shutdown**: Proper shutdown handling with configurable timeout
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
export FAIRWINDS_TOKEN=your-api-token
./insights-event-watcher \
  --log-source=local \
  --insights-host=https://insights.fairwinds.com \
  --organization=my-org \
  --cluster=production
```

#### CloudWatch Mode (EKS Clusters)
```bash
# Watch policy violations from EKS CloudWatch logs
export FAIRWINDS_TOKEN=your-api-token
./insights-event-watcher \
  --log-source=cloudwatch \
  --cloudwatch-log-group=/aws/eks/production-eks/cluster \
  --cloudwatch-region=us-west-2 \
  --cloudwatch-filter-pattern="{ $.stage = \"ResponseComplete\" && $.responseStatus.code >= 400 }" \
  --insights-host=https://insights.fairwinds.com \
  --organization=my-org \
  --cluster=production
```

### Environment Variables

The watcher uses environment variables for sensitive configuration, following Fairwinds security best practices:

- `FAIRWINDS_TOKEN`: Fairwinds Insights API token (required for Insights integration)
- `FAIRWINDS_INSIGHTS_HOST`: Fairwinds Insights hostname (alternative to --insights-host)
- `FAIRWINDS_ORG`: Fairwinds organization name (alternative to --organization)  
- `FAIRWINDS_CLUSTER`: Cluster name (alternative to --cluster)

**Security Note**: Never pass API tokens via command-line arguments as they may be visible in process lists. Always use environment variables or Kubernetes secrets.

### Command Line Options

#### General Options
- `--log-level`: Log level - debug, info, warn, error (default: `info`)
- `--insights-host`: Fairwinds Insights hostname (optional)
- `--organization`: Fairwinds organization name (required if insights-host provided)
- `--cluster`: Cluster name (required if insights-host provided)
- `FAIRWINDS_TOKEN`: Fairwinds Insights API token environment variable (required if insights-host provided)
- `--event-buffer-size`: Size of the event processing buffer (default: `10000`)
- `--http-timeout-seconds`: HTTP client timeout in seconds (default: `30`)
- `--rate-limit-per-minute`: Maximum API calls per minute (default: `60`)

#### Performance & Monitoring Options
- **Backpressure Handling**: Automatically retries when event channel is full (3 retries, 100ms delay)
- **Metrics Logging**: Periodic logging of processing rates and dropped events (every 30 seconds)
- **Channel Utilization**: Monitors event channel capacity and utilization

#### Health Check Options
- **`--health-port`**: Port for health check endpoints (default: 8080)
- **`--shutdown-timeout`**: Graceful shutdown timeout (default: 30s)

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

## Performance & Monitoring

### Backpressure Handling

The watcher implements intelligent backpressure handling to prevent event loss when the event channel is full:

- **Automatic Retries**: When the event channel is full, the watcher automatically retries up to 3 times with 100ms delays
- **Graceful Degradation**: If all retries fail, events are dropped with detailed logging for monitoring
- **Configurable Parameters**: Retry count, delay, and metrics logging intervals can be customized

### Metrics & Monitoring

The watcher provides comprehensive metrics for monitoring performance:

#### Key Metrics Tracked
- **Events Processed**: Total number of events successfully processed
- **Events Dropped**: Total number of events dropped due to backpressure
- **Channel Utilization**: Percentage of event channel capacity in use
- **Processing Rate**: Events processed per second
- **Dropped Events Rate**: Events dropped per second
- **Processing Duration**: Time taken to process individual events

#### Metrics Logging
Metrics are automatically logged every 30 seconds with the following information:
```
INFO[2024-01-15T10:30:00Z] Watcher metrics events_processed=1250 events_dropped=5 events_in_channel=12 channel_capacity=1000 channel_utilization=1.2 events_per_second=41.7 processing_rate=42.1 dropped_events_rate=0.2 uptime=30s
```

#### Monitoring Recommendations
- **Channel Utilization**: Keep below 80% for optimal performance
- **Dropped Events**: Monitor for increases indicating backpressure issues
- **Processing Rate**: Track for performance degradation over time

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

## Health Checks & Operations

### Health Check Endpoints

The watcher provides HTTP endpoints for Kubernetes health checks:

- **`/healthz`** - Liveness probe endpoint
  - Returns HTTP 200 when the process is running
  - Returns HTTP 503 when the process is stopping
  - Used by Kubernetes to determine if the pod should be restarted

- **`/readyz`** - Readiness probe endpoint
  - Returns HTTP 200 when the watcher is ready to process events
  - Returns HTTP 503 when the watcher is not ready (e.g., still starting up)
  - Used by Kubernetes to determine if the pod should receive traffic

- **`/health`** - General health endpoint
  - Returns comprehensive health information including watcher status
  - Includes details about registered health checkers
  - Useful for monitoring and debugging

### Health Check Configuration

```bash
# Configure health check port (default: 8080)
--health-port=8080

# Configure graceful shutdown timeout (default: 30s)
--shutdown-timeout=30s
```

### Graceful Shutdown

The watcher implements proper graceful shutdown:

1. **Signal Handling**: Responds to SIGTERM and SIGINT signals
2. **Timeout Protection**: Configurable shutdown timeout prevents hanging
3. **Resource Cleanup**: Stops all watchers and closes connections properly
4. **Health Check Shutdown**: Stops health check server gracefully

### Kubernetes Integration

The Helm chart includes proper health check configuration:

```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /readyz
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
  timeoutSeconds: 3
  failureThreshold: 3
```


#### Example PolicyViolation Events

```bash
# These events will be captured and sent to Insights API:
kubectl get events | grep -E "PolicyViolation|VAPViolation" | grep "(blocked)"

# ValidatingAdmissionPolicy format:
Warning   PolicyViolation     validatingadmissionpolicy/disallow-host-path   Deployment default/nginx: [disallow-host-path] fail (blocked); HostPath volumes are forbidden...

# Regular Kyverno policy format:
Warning   PolicyViolation     deployment/nginx                               policy disallow-host-path/disallow-host-path fail (blocked): HostPath volumes are forbidden...

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
        - "--audit-log-path=/var/log/kubernetes/kube-apiserver-audit.log"
        env:
        - name: FAIRWINDS_TOKEN
          valueFrom:
            secretKeyRef:
              name: insights-token-secret
              key: token
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
        env:
        - name: FAIRWINDS_TOKEN
          valueFrom:
            secretKeyRef:
              name: insights-token-secret
              key: token
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
