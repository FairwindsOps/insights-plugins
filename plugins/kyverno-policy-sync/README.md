# Kyverno Policy Sync

A standalone Kubernetes service that synchronizes Kyverno policies between Fairwinds Insights and Kubernetes clusters.

## Overview

The Kyverno Policy Sync service automatically keeps your cluster's Kyverno policies in sync with the policies defined in Fairwinds Insights. It can run as a CronJob for scheduled synchronization or be triggered on-demand when policies change.

## Features

- **Automatic Policy Synchronization**: Keeps cluster policies in sync with Insights
- **Insights-Managed Only**: Only affects policies with `insights.fairwinds.com/owned-by: "Fairwinds Insights"` annotation
- **Distributed Locking**: Uses Kubernetes ConfigMap for preventing concurrent operations across pods
- **Kyverno CLI Integration**: Uses `kyverno apply` and `kyverno delete` for all policy operations
- **Policy Validation**: Validates policies using Kyverno CLI before applying
- **Dry-Run Mode**: Preview changes before applying them
- **Comprehensive Logging**: Detailed audit trail of all operations
- **Flexible Deployment**: Can run as CronJob or on-demand

## Architecture

### Primary Use Case: CronJob
The service is designed to run as a Kubernetes CronJob that periodically synchronizes policies:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: kyverno-policy-sync
spec:
  schedule: "*/15 * * * *"  # Every 15 minutes
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: kyverno-policy-sync
            image: quay.io/fairwinds/kyverno-policy-sync:latest
            env:
            - name: FAIRWINDS_INSIGHTS_HOST
              value: "https://insights.fairwinds.com"
            - name: FAIRWINDS_TOKEN
              valueFrom:
                secretKeyRef:
                  name: insights-token
                  key: token
            - name: FAIRWINDS_ORG
              value: "your-org"
            - name: FAIRWINDS_CLUSTER
              value: "your-cluster"
```

### Secondary Use Case: On-Demand
The service can also be triggered on-demand by the on-demand job runner when policies change in Insights.

## Configuration

### Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `FAIRWINDS_INSIGHTS_HOST` | Insights API host | - | Yes |
| `FAIRWINDS_TOKEN` | Insights API token | - | Yes |
| `FAIRWINDS_ORG` | Organization name | - | Yes |
| `FAIRWINDS_CLUSTER` | Cluster name | - | Yes |
| `FAIRWINDS_DEV_MODE` | Enable dev mode | `false` | No |
| `DRY_RUN` | Enable dry-run mode | `false` | No |
| `SYNC_INTERVAL` | Sync interval for continuous mode | `15m` | No |
| `LOCK_TIMEOUT` | Lock timeout duration | `30m` | No |
| `VALIDATE_POLICIES` | Validate policies before applying | `true` | No |
| `LOG_LEVEL` | Log level (debug, info, warn, error) | `info` | No |
| `ONE_SHOT` | Run once and exit | `false` | No |

### Configuration File

You can also use a YAML configuration file:

```yaml
host: "https://insights.fairwinds.com"
token: "your-token"
organization: "your-org"
cluster: "your-cluster"
devMode: false
dryRun: false
syncInterval: "15m"
lockTimeout: "30m"
validatePolicies: true
logLevel: "info"
```

## Deployment

### As CronJob

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: kyverno-policy-sync
  namespace: default
spec:
  schedule: "*/15 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          serviceAccountName: kyverno-policy-sync
          containers:
          - name: kyverno-policy-sync
            image: quay.io/fairwinds/kyverno-policy-sync:latest
            env:
            - name: FAIRWINDS_INSIGHTS_HOST
              value: "https://insights.fairwinds.com"
            - name: FAIRWINDS_TOKEN
              valueFrom:
                secretKeyRef:
                  name: insights-token
                  key: token
            - name: FAIRWINDS_ORG
              value: "your-org"
            - name: FAIRWINDS_CLUSTER
              value: "your-cluster"
            - name: DRY_RUN
              value: "false"
            - name: VALIDATE_POLICIES
              value: "true"
          restartPolicy: OnFailure
```

### As Deployment (Continuous Mode)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kyverno-policy-sync
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kyverno-policy-sync
  template:
    metadata:
      labels:
        app: kyverno-policy-sync
    spec:
      serviceAccountName: kyverno-policy-sync
      containers:
      - name: kyverno-policy-sync
        image: quay.io/fairwinds/kyverno-policy-sync:latest
        env:
        - name: FAIRWINDS_INSIGHTS_HOST
          value: "https://insights.fairwinds.com"
        - name: FAIRWINDS_TOKEN
          valueFrom:
            secretKeyRef:
              name: insights-token
              key: token
        - name: FAIRWINDS_ORG
          value: "your-org"
        - name: FAIRWINDS_CLUSTER
          value: "your-cluster"
        - name: SYNC_INTERVAL
          value: "15m"
```

### RBAC Configuration

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kyverno-policy-sync
  namespace: default
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kyverno-policy-sync
rules:
- apiGroups: ["kyverno.io"]
  resources: ["clusterpolicies", "policies"]
  verbs: ["get", "list", "create", "update", "delete"]
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "list", "create", "update", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kyverno-policy-sync
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: kyverno-policy-sync
subjects:
- kind: ServiceAccount
  name: kyverno-policy-sync
  namespace: default
```

## How It Works

1. **Fetch Policies**: Retrieves expected policies from Insights API
2. **Compare States**: Compares expected policies with currently deployed policies
3. **Determine Actions**: Identifies policies to apply, update, or remove
4. **Acquire Lock**: Uses distributed locking to prevent concurrent operations
5. **Validate Policies**: Validates policies using Kyverno CLI (if enabled)
6. **Execute Changes**: Applies, updates, or removes policies as needed
7. **Release Lock**: Releases the distributed lock
8. **Report Results**: Logs comprehensive results of the sync operation

## Policy Management

### Policy Ownership

Only policies with the following annotation are managed by the sync:

```yaml
metadata:
  annotations:
    insights.fairwinds.com/owned-by: "Fairwinds Insights"
```

### Policy Operations

The sync performs the following operations using Kyverno CLI:

1. **Apply**: New policies from Insights that don't exist in the cluster
   ```bash
   kyverno apply policy.yaml
   ```

2. **Update**: Existing policies that have changed in Insights
   ```bash
   kyverno apply policy.yaml  # kyverno apply handles both create and update
   ```

3. **Remove**: Policies that exist in the cluster but are no longer in Insights
   ```bash
   kyverno delete clusterpolicy policy-name
   ```

## Distributed Locking

The sync uses Kubernetes ConfigMap for distributed locking to prevent concurrent operations:

### Lock Mechanism
- **ConfigMap Name**: `kyverno-policy-sync-lock`
- **Namespace**: Current pod namespace (or `default`)
- **Lock Data**: Contains `locked-by`, `locked-at`, and `lock-timeout` information
- **Stale Lock Detection**: Automatically removes locks older than the timeout period

### Lock Operations
```bash
# Check lock status
kubectl get configmap kyverno-policy-sync-lock -n <namespace>

# Manual lock release
kubectl delete configmap kyverno-policy-sync-lock -n <namespace>
```

## Monitoring

The sync provides detailed logging for monitoring:

```json
{
  "level": "info",
  "msg": "Policy sync completed",
  "success": true,
  "duration": "5.2s",
  "summary": "Applied 2, Updated 1, Removed 0, Failed 0",
  "applied": ["policy1", "policy2"],
  "updated": ["policy3"],
  "removed": [],
  "failed": []
}
```

## Troubleshooting

### Common Issues

1. **Lock ConfigMap Exists**: Another sync operation is running
   - Solution: Wait for completion or remove stale lock ConfigMap
   ```bash
   kubectl delete configmap kyverno-policy-sync-lock -n <namespace>
   ```

2. **Policy Validation Failed**: Policy has syntax errors
   - Solution: Check policy syntax and fix errors

3. **API Authentication Failed**: Invalid token or permissions
   - Solution: Verify token and permissions

4. **Kubernetes API Errors**: Cluster connectivity issues
   - Solution: Check cluster connectivity and RBAC permissions

### Debug Mode

Enable debug logging for detailed troubleshooting:

```bash
export LOG_LEVEL="debug"
```

## Development

### Building

```bash
cd plugins/kyverno-policy-sync
go build -o kyverno-policy-sync ./cmd/main.go
```

### Testing

```bash
cd plugins/kyverno-policy-sync
go test ./pkg/sync/... -v
```

### Docker

```bash
cd plugins/kyverno-policy-sync
docker build -t kyverno-policy-sync .
```

## Integration with On-Demand Job Runner

The standalone plugin can also be integrated with the on-demand job runner for immediate policy synchronization when policies change in Insights. This provides the best of both worlds:

- **Scheduled Sync**: Regular synchronization via CronJob
- **Immediate Sync**: On-demand synchronization when policies change

The on-demand job runner can trigger the standalone plugin by setting `ONE_SHOT=true` and running it as a Kubernetes Job.
