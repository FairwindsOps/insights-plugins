# network-flow

DaemonSet plugin that captures Kubernetes network observations via Inspektor Gadget and forwards `FlowEventBatch` messages to `network-flow-aggregator` over gRPC.

Two event kinds are emitted per TCP connection:

- **CONNECT** — from `trace_tcp` (connect/disconnect lifecycle)
- **TRAFFIC** — from `top_tcp` (byte delta snapshots for active connections)

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
