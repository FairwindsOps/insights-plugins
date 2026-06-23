# network-flow-aggregator

Deployment plugin that ingests `FlowEventBatch` messages from `network-flow` agents, enriches each event with Kubernetes workload and service metadata, maintains an IP-to-hostname cache from DNS responses, and stores individual `EnrichedFlowEvent` records in memory.

## Running locally

```bash
go run ./pkg -grpc-addr=:4317 -http-addr=:8080
```

Debug HTTP endpoints (when running): `/healthz`, `/api/v1/flows`.

### Flow export API

```
GET /api/v1/flows?since=<timestamp_unix_nano>&limit=1000&offset=0&namespace=insights&event_kind=CONNECT
```

Query parameters:

| Param | Description |
|---|---|
| `since` | Return events with timestamp strictly after this value (unix nano) |
| `limit` | Maximum events to return |
| `offset` | Skip first N matching events |
| `namespace` | Filter by source or destination namespace |
| `event_kind` | `CONNECT`, `TRAFFIC`, `DNS_QUERY`, or `DNS_RESPONSE` |
| `src_workload_kind` | Filter by enriched source workload kind |
| `dst_kind` | Filter by enriched destination kind (e.g. `Service`, `ExternalHostname`) |

Response: `{ "events": [EnrichedFlowEvent...], "count": N }`

A future backend should poll this API (or replace it with Timescale ingestion) and own all aggregation — servicemap edges, analytics, long-term retention.

### DNS observability

DNS responses from `trace_dns` populate an in-memory IP-to-hostname cache (TTL matches `-max-age`). When enriching TCP flows, destination resolution order is:

1. Kubernetes Service (ClusterIP / EndpointSlice)
2. Workload-scoped DNS cache (`namespace` + `pod` + IP)
3. Cluster-scoped DNS cache (IP only)

External destinations resolve to `dst_ref.kind = ExternalHostname` with the queried hostname (e.g. `api.stripe.com`).

## Insights upstream

When configured, the collector forwards enriched events to the Insights API over gRPC after local enrichment.

| Flag | Env | Description |
|---|---|---|
| `-insights-grpc-addr` | `INSIGHTS_GRPC_ADDR` | Insights network flow gRPC address |
| `-organization` | `ORGANIZATION` | Insights organization slug |
| `-cluster` | `CLUSTER` | Insights cluster name |
| `-auth-token` | `AUTH_TOKEN` | Insights cluster auth token |

All four upstream settings are required when `INSIGHTS_GRPC_ADDR` is set.

### Retention flags

| Flag | Env | Default | Description |
|---|---|---|---|
| `-max-events` | `MAX_EVENTS` | `100000` | Ring buffer capacity |
| `-max-age` | `MAX_AGE` | `15m` | Drop events older than this |

## How to run it
```
export CLUSTER_TOKEN=
export ORGANIZATION=
export CLUSTER=

AUTH_TOKEN=$CLUSTER_TOKEN \
INSIGHTS_GRPC_ADDR=host.docker.internal:4318 \
ORGANIZATION=$ORGANIZATION \
CLUSTER=$CLUSTER \
KEEP_GOING=1 \
ENABLE_INSIGHTS_UPSTREAM=true \
./test/network-flow-e2e/kind-e2e.sh all
```