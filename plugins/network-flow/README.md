# network-flow

DaemonSet plugin that captures Kubernetes network observations via Inspektor Gadget and forwards `FlowEventBatch` messages to `network-flow-aggregator` over gRPC.

Three event kinds are emitted:

- **CONNECT** — from `trace_tcp` (connect/disconnect lifecycle)
- **TRAFFIC** — from `top_tcp` (byte delta snapshots for active connections)
- **DNS_QUERY** / **DNS_RESPONSE** — from `trace_dns` (hostname resolution queries and responses)

The agent gRPC client uses the **agent–aggregator** contract defined in `network-flow-aggregator` (`aggregator.v1.AgentIngest`). See that plugin's README for proto layout and codegen.

## Configuration

| Flag | Env | Default | Description |
|---|---|---|---|
| `-batch-size` | `BATCH_SIZE` | `1000` | Events per gRPC send batch |
| `-max-pending-events` | `MAX_PENDING_EVENTS` | `50000` | Pending queue capacity (drop-oldest) |
| `-flush-interval` | `FLUSH_INTERVAL` | `15s` | Max time between sends |

When the pending queue exceeds `-max-pending-events`, the oldest events are dropped. Sustained overload emits rate-limited `pending flow events dropped by retention` warnings. Size the cap for your node memory limit (the e2e DaemonSet uses a 512Mi limit; 50k events is a conservative default).

## Running locally

Build binaries (linux only for the agent and entrypoint):

```bash
CGO_ENABLED=0 GOOS=linux go build -o bin/network-flow ./pkg
CGO_ENABLED=0 GOOS=linux go build -o bin/entrypoint ./cmd/entrypoint
```

End-to-end testing against a kind cluster:

```bash
./test/network-flow-e2e/kind-e2e.sh all
```
