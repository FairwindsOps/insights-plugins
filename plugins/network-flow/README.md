# network-flow

DaemonSet plugin that captures Kubernetes network observations via Inspektor Gadget and forwards `FlowEventBatch` messages to `network-flow-aggregator` over gRPC.

Three event kinds are emitted:

- **CONNECT** — from `trace_tcp` (connect/disconnect lifecycle)
- **TRAFFIC** — from `top_tcp` (byte delta snapshots for active connections)
- **DNS_QUERY** / **DNS_RESPONSE** — from `trace_dns` (hostname resolution queries and responses)

The agent gRPC client uses the **agent–aggregator** contract defined in `network-flow-aggregator` (`aggregator.v1.AgentIngest`). See that plugin's README for proto layout and codegen.

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
