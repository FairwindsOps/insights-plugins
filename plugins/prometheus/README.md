# Prometheus Collector

This is a utility for collecting performance metrics from Prometheus to send to Fairwinds Insights.

## Configuration

The collector is configured via environment variables:

| Environment Variable | Required | Description |
|---------------------|----------|-------------|
| `PROMETHEUS_ADDRESS` | Yes | The address of your Prometheus-compatible server |
| `PROMETHEUS_BEARER_TOKEN` | No | Bearer token for authentication |
| `PROMETHEUS_TENANT_ID` | No | Tenant ID for multi-tenant backends (e.g., Grafana Mimir) |
| `CLUSTER_NAME` | No | Name of the cluster to filter metrics |
| `SKIP_NON_ZERO_METRICS_CHECK` | No | Skip validation for cAdvisor metrics |
| `SKIP_KSM_NON_ZERO_METRICS_CHECK` | No | Skip validation for kube-state-metrics |
| `LOGRUS_LEVEL` | No | Log level (trace, debug, info, warning, error, fatal, panic) |

## Standard Prometheus Usage

For a standard Prometheus deployment:

```bash
export PROMETHEUS_ADDRESS="http://prometheus-server:9090"
```

## Grafana Mimir / Multi-Tenant Support

This collector supports [Grafana Mimir](https://grafana.com/oss/mimir/) and other multi-tenant Prometheus-compatible backends that use the `X-Scope-OrgID` header for tenant identification.

### Configuration

Set the `PROMETHEUS_TENANT_ID` environment variable to your tenant ID:

```bash
export PROMETHEUS_ADDRESS="https://mimir.example.com/prometheus"
export PROMETHEUS_TENANT_ID="my-tenant-id"
```

### Address Path Handling

When using Mimir, provide the full base path to the Prometheus API endpoint. The collector will automatically append `/api/v1` for API calls:

- ✅ `https://mimir.example.com/prometheus` → queries will go to `/prometheus/api/v1/query`
- ✅ `https://mimir.example.com` → queries will go to `/api/v1/query`
- ❌ `https://mimir.example.com/prometheus/api/v1` → Don't include `/api/v1`

### With Authentication

For Mimir deployments requiring authentication:

```bash
export PROMETHEUS_ADDRESS="https://mimir.example.com/prometheus"
export PROMETHEUS_TENANT_ID="my-tenant-id"
export PROMETHEUS_BEARER_TOKEN="your-api-token"
```

## Using the Client Library

The `data` package can also be used as a library in your own Go projects.

### Basic Usage

```go
import "github.com/fairwindsops/insights-plugins/plugins/prometheus/pkg/data"

// Standard Prometheus (backward compatible)
client, err := data.GetClient("http://prometheus:9090", "")

// With bearer token (backward compatible)
client, err := data.GetClient("http://prometheus:9090", "my-token")
```

### With Functional Options (Recommended)

```go
import "github.com/fairwindsops/insights-plugins/plugins/prometheus/pkg/data"

// Standard Prometheus
client, err := data.GetClientWithOptions("http://prometheus:9090")

// With bearer token
client, err := data.GetClientWithOptions("http://prometheus:9090",
    data.WithBearerToken("my-token"))

// Grafana Mimir with tenant ID
client, err := data.GetClientWithOptions("https://mimir.example.com/prometheus",
    data.WithTenantID("my-tenant"))

// Mimir with tenant ID and bearer token
client, err := data.GetClientWithOptions("https://mimir.example.com/prometheus",
    data.WithBearerToken("my-token"),
    data.WithTenantID("my-tenant"))
```

### Available Options

| Option | Description |
|--------|-------------|
| `WithBearerToken(token string)` | Sets the bearer token for authentication |
| `WithTenantID(tenantID string)` | Sets the tenant ID header (`X-Scope-OrgID`) for multi-tenant backends |

## Google Cloud Monitoring

For Google Cloud Monitoring (Managed Service for Prometheus), the collector automatically obtains an access token using Application Default Credentials when the address contains `monitoring.googleapis.com`.

```bash
export PROMETHEUS_ADDRESS="https://monitoring.googleapis.com/v1/projects/YOUR_PROJECT/location/global/prometheus"
```
