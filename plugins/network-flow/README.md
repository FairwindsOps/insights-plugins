# network-flow

DaemonSet plugin that captures Kubernetes network flows via Inspektor Gadget and forwards raw `NetworkFlow` batches to `network-flow-aggregator` over gRPC.

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
