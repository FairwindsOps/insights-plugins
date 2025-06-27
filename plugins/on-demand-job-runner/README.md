# on-demand-job-runner

## Overview

The `on-demand-job-runner` service is responsible for claiming and executing **on-demand report jobs**, which are triggered by the Insights user interface.
Each job is dispatched to the same **Kubernetes cluster** from which it was triggered. Additionally, report jobs can accept `options` — these are key-value fields that are automatically converted into **environment variables** and injected into the spawned job container.

---

## Configuration

To run the service, you must provide a configuration file. A sample config is available in `.config.yaml`:

```yaml
organization: "example_org"
cluster: "example_cluster"
token: "your_decoded_token_here"
host: "https://example.com"
maxConcurrentJobs: 10
devMode: true
```

Alternatively, you can define these values using **environment variables**:

```bash
export ORGANIZATION="example_org"
export CLUSTER="example_cluster"
export TOKEN="your_decoded_token_here"
export HOST="https://example.com"
export MAX_CONCURRENT_JOBS=10
export DEV_MODE=true
```

> ℹ️ **Note:** Environment variables take precedence over values defined in `.config.yaml`.

---

## Running the Application

To start the application:

```bash
go run main.go
```

---

## Running Locally

When running locally, it's recommended to specify the Kubernetes namespace explicitly using the `NAMESPACE` environment variable:

```bash
NAMESPACE=insights-agent go run main.go
```

### Using the Mock Insights Client

To simulate interactions and assist in debugging, enable the mock client:

```bash
MOCK_INSIGHTS_CLIENT=true NAMESPACE=insights-agent go run main.go
```
