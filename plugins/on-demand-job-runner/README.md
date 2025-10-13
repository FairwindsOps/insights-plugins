# On-Demand Job Runner

The On-Demand Job Runner is a Kubernetes service that processes on-demand jobs from Fairwinds Insights. It supports various report types including the new Kyverno Policy Sync functionality.

## Features

- **On-Demand Job Processing**: Processes various types of on-demand jobs from Insights
- **Kyverno Policy Sync**: Synchronizes Kyverno policies between Insights and Kubernetes clusters
- **Concurrency Control**: Prevents simultaneous execution of the same job types
- **Comprehensive Logging**: Detailed logging for monitoring and debugging
- **Dry-Run Mode**: Preview changes before applying them
- **Policy Validation**: Validates policies using Kyverno CLI before applying

## Supported Job Types

- `trivy` - Container image vulnerability scanning
- `cloudcosts` - Cloud cost analysis
- `falco` - Runtime security monitoring
- `nova` - Container image vulnerability scanning
- `pluto` - Kubernetes version compatibility checking
- `polaris` - Kubernetes best practices validation
- `prometheus-metrics` - Prometheus metrics collection
- `goldilocks` - Resource right-sizing recommendations
- `rbac-reporter` - RBAC analysis and reporting
- `right-sizer` - Resource right-sizing analysis
- `workloads` - Workload analysis and reporting
- `kube-hunter` - Kubernetes penetration testing
- `kube-bench` - CIS Kubernetes benchmark testing
- `kyverno` - Kyverno policy violation reporting
- `kyverno-policy-sync` - **NEW** Kyverno policy synchronization
- `gonogo` - Go module dependency analysis

## Kyverno Policy Sync

The Kyverno Policy Sync functionality allows automatic synchronization of Kyverno policies between Fairwinds Insights and Kubernetes clusters.

### How It Works

1. **Fetch Policies**: Retrieves expected policies from Insights API
2. **Compare States**: Compares expected policies with currently deployed policies
3. **Determine Actions**: Identifies policies to apply, update, or remove
4. **Validate Policies**: Validates policies using Kyverno CLI (if enabled)
5. **Execute Changes**: Applies, updates, or removes policies as needed
6. **Report Results**: Logs comprehensive results of the sync operation

### Key Features

- **Insights-Managed Only**: Only affects policies with `insights.fairwinds.com/owned-by: "Fairwinds Insights"` annotation
- **Atomic Operations**: Fails fast if any operation fails
- **Dry-Run Support**: Preview changes before applying them
- **Concurrency Control**: File-based locking prevents simultaneous sync operations
- **Policy Validation**: Uses Kyverno CLI to validate policies before applying
- **Comprehensive Logging**: Detailed audit trail of all operations

### Configuration

The Kyverno Policy Sync can be configured via environment variables:

- `DRY_RUN`: Set to `true` to enable dry-run mode (default: `false`)
- `VALIDATE_POLICIES`: Set to `false` to disable policy validation (default: `true`)
- `SYNC_INTERVAL`: Sync interval for cronjob (default: `15m`)
- `LOCK_TIMEOUT`: Lock timeout for preventing concurrent operations (default: `30m`)

### API Endpoints

The sync functionality uses the following Insights API endpoints:

- `GET /v0/organizations/{organization}/clusters/{cluster}/kyverno-policies/with-app-groups-applied/yaml`

### Usage

#### As On-Demand Job

The sync can be triggered on-demand by Insights when policies change:

```bash
# Environment variables required
export FAIRWINDS_INSIGHTS_HOST="https://insights.fairwinds.com"
export FAIRWINDS_TOKEN="your-token"
export FAIRWINDS_ORG="your-org"
export FAIRWINDS_CLUSTER="your-cluster"

# Optional configuration
export DRY_RUN="false"
export VALIDATE_POLICIES="true"
```

#### As CronJob

The sync can also run as a scheduled CronJob:

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
            image: quay.io/fairwinds/on-demand-job-runner:latest
            env:
            - name: JOB_TYPE
              value: "kyverno-policy-sync"
            - name: DRY_RUN
              value: "false"
            - name: VALIDATE_POLICIES
              value: "true"
```

### Policy Management

#### Policy Ownership

Only policies with the following annotation are managed by the sync:

```yaml
metadata:
  annotations:
    insights.fairwinds.com/owned-by: "Fairwinds Insights"
```

#### Policy Operations

The sync performs the following operations:

1. **Apply**: New policies from Insights that don't exist in the cluster
2. **Update**: Existing policies that have changed in Insights
3. **Remove**: Policies that exist in the cluster but are no longer in Insights

#### Policy Validation

Before applying policies, the sync validates them using Kyverno CLI:

```bash
kyverno apply policy.yaml --dry-run
```

### Error Handling

The sync implements comprehensive error handling:

- **Atomic Operations**: If any operation fails, the entire sync fails
- **Detailed Logging**: All operations are logged with context
- **Graceful Degradation**: Continues with other policies if one fails
- **Lock Management**: Automatic cleanup of stale locks

### Monitoring

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

### Troubleshooting

#### Common Issues

1. **Lock File Exists**: Another sync operation is running
   - Solution: Wait for completion or remove stale lock file

2. **Policy Validation Failed**: Policy has syntax errors
   - Solution: Check policy syntax and fix errors

3. **API Authentication Failed**: Invalid token or permissions
   - Solution: Verify token and permissions

4. **Kubernetes API Errors**: Cluster connectivity issues
   - Solution: Check cluster connectivity and RBAC permissions

#### Debug Mode

Enable debug logging for detailed troubleshooting:

```bash
export LOG_LEVEL="debug"
```

### Security Considerations

- **RBAC**: Ensure proper RBAC permissions for policy management
- **Network**: Secure communication with Insights API
- **Secrets**: Protect authentication tokens
- **Validation**: Always validate policies before applying

### Development

#### Running Tests

```bash
cd plugins/on-demand-job-runner
go test ./pkg/kyverno/...
```

#### Building

```bash
cd plugins/on-demand-job-runner
go build -o on-demand-job-runner ./cmd/main.go
```

#### Docker

```bash
cd plugins/on-demand-job-runner
docker build -t on-demand-job-runner .
```