# network-flow-aggregator

Deployment plugin that ingests flow batches from `network-flow` agents, enriches them with Kubernetes service and workload metadata, and aggregates service map edges in memory.

## Running locally

```bash
go run ./pkg -grpc-addr=:4317 -http-addr=:8080
```

Debug HTTP endpoints (when running): `/healthz`, `/api/v1/servicemap`.
