# network-flow-aggregator

Deployment plugin that ingests `FlowEventBatch` messages from `network-flow` agents, enriches each event with Kubernetes workload and service metadata, maintains an IP-to-hostname cache from DNS responses, and stores individual `EnrichedFlowEvent` records in memory.

## gRPC contract (agent ↔ aggregator)

This plugin owns the **agent–aggregator** protobuf contract. Source files live under `proto/aggregator/v1/`; generated Go is committed under `pkg/aggregator/v1` (`aggregv1`).

| Item | Value |
|---|---|
| Package | `aggregator.v1` |
| Service | `AgentIngest` |
| RPC | `PushEvents(stream FlowEventBatch) returns (PushAck)` |
| Consumers | `network-flow` agent (client), this collector (server) |

The **aggregator–API** contract (`NetworkFlowIngest.PushEnrichedEvents`) is owned by [fairwinds-insights](https://github.com/FairwindsOps/Insights) under `api/proto/api/v1/`. A local copy lives under `proto/insights/api/v1/`; generated Go is committed under `pkg/insights/v1` (`insightsv1`).

### Regenerating Go from proto (agent ↔ aggregator)

Note: In an ideal setup, this plugin would consume the Insights API protos as a published module. Today that contract lives in the private fairwinds-insights repo and is not available as an external proto package, so we vendor a local copy under proto/insights/ and commit the generated Go stubs.

Requires `protoc` on your PATH (`brew install protobuf`). Plugin versions are pinned in the script.

```bash
./scripts/generate-proto.sh
```

Edit `proto/aggregator/v1/*.proto`, run the script, and commit both the `.proto` files and `pkg/aggregator/v1/*.go`.

### Syncing Insights API protos (aggregator ↔ API)

Requires a local [fairwinds-insights](https://github.com/FairwindsOps/Insights) checkout. Copies protos from that repo and regenerates `pkg/insights/v1`.

```bash
./scripts/generate-insights-api-proto.sh --fairwinds-insights-path ../../../Insights
```

Commit both `proto/insights/` and `pkg/insights/v1/` after syncing.

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
| `dst_kind` | Filter by enriched destination kind (e.g. `Service`, `Deployment`, `ExternalHostname`) |

Response: `{ "events": [EnrichedFlowEvent...], "count": N }`

A future backend should poll this API (or replace it with Timescale ingestion) and own all aggregation — servicemap edges, analytics, long-term retention.

### DNS observability

DNS responses from `trace_dns` populate an in-memory IP-to-hostname cache (TTL matches `-max-age`). When enriching TCP flows, destination resolution order is:

1. Kubernetes Service (ClusterIP / EndpointSlice `(IP, port)`)
2. Pod IP (`IP` only, any port) rolled up to top-controller workload
3. Workload-scoped DNS cache (`namespace` + `pod` + IP)
4. Cluster-scoped DNS cache (IP only)

Pod-IP resolution skips `hostNetwork` pods and IPs shared by multiple distinct pods (ambiguous); those remain unlabeled and appear as `External` in downstream aggregates. External destinations resolve to `dst_ref.kind = ExternalHostname` with the queried hostname (e.g. `api.stripe.com`).

### Service backend attribution

For TCP flows where `dst_ref.kind = Service`, the collector attempts to identify which backend pod/workload received the connection:

1. **Direct pod IP:** when the client connects to an EndpointSlice address, resolve the pod from `TargetRef` and roll up to its top-controller workload.
2. **ClusterIP correlation:** index server-side events whose `(src_addr, src_port)` matches a ready endpoint; map the client peer `(dst_addr, dst_port)` to that backend for later client→Service events on the same `(src_addr, src_port)`.

Backend identity is attached only on **client→Service** rows (`backend_workload`, `backend_pod` on `EnrichedFlowEvent`). Server-side rows are correlation inputs only; byte totals on Service→backend edges use client-side TRAFFIC bytes to avoid double-counting.

**Limitations:** SNAT/NodePort/external clients, missing agents on backend nodes, ephemeral port reuse, hostNetwork/node-IP traffic (e.g. `kubernetes` Service on node addresses, DaemonSets sharing a node IP), and ambiguous pod-IP ownership may leave destination or backend fields empty. The collector does not invent replicas or spread bytes evenly across backends.

## Insights upstream

When configured, the collector forwards enriched events to the Insights API over gRPC after local enrichment.

| Flag | Env | Description |
|---|---|---|
| `-insights-grpc-addr` | `INSIGHTS_GRPC_ADDR` | Insights network flow gRPC address |
| `-insights-grpc-tls` | `INSIGHTS_GRPC_TLS` | TLS mode: `auto` (default), `true`, or `false` |
| `-insights-grpc-tls-server-name` | `INSIGHTS_GRPC_TLS_SERVER_NAME` | TLS server name; defaults to hostname from addr |
| `-insights-grpc-tls-ca-file` | `INSIGHTS_GRPC_TLS_CA_FILE` | Optional PEM file with extra CA certs |
| `-organization` | `ORGANIZATION` | Insights organization slug |
| `-cluster` | `CLUSTER` | Insights cluster name |
| `-auth-token` | `AUTH_TOKEN` | Insights cluster auth token |

All four upstream identity settings are required when `INSIGHTS_GRPC_ADDR` is set.

TLS is enabled automatically when the address uses port `443` or an `https://` prefix. For ALB-terminated gRPC (e.g. `grpc.staging.insights.fairwinds.com:443`), set the public hostname as the address and leave TLS on `auto`.

### Retention flags

| Flag | Env | Default | Description |
|---|---|---|---|
| `-max-events` | `MAX_EVENTS` | `100000` | Ring buffer capacity |
| `-max-age` | `MAX_AGE` | `15m` | Drop events older than this |

Retention applies to both the debug HTTP API and upstream delivery to Insights. Events evicted before they are sent upstream are not forwarded; sustained backpressure emits rate-limited warnings when unsent events are dropped.

## How to run it
```
export AUTH_TOKEN=
export INSIGHTS_GRPC_ADDR=host.docker.internal:4318
export ORGANIZATION=
export CLUSTER=

./test/network-flow-e2e/kind-e2e.sh all
```

### Demo cluster verification (Service→backend)

1. Deploy a frontend→backend Service with at least two backend replicas.
2. Confirm enriched client→Service TRAFFIC events include `backend_workload` and `backend_pod`; server-side peer events do not.
3. Query Insights `GET .../network-observability/service-map` — Workload→Service byte totals unchanged.
4. Query `GET .../network-observability/service-backends` — per-backend byte split matches client-side raw event sums for the same window.
5. Stop the network-flow agent on one backend node and confirm unattributed Service traffic remains possible (empty backend fields, no fabricated split).