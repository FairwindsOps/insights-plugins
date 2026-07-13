# Changelog

## 0.0.8
* Bump library github.com/inspektor-gadget/inspektor-gadget to v0.54.0
* Bump indirect libraries dependencies

## 0.0.7
* Bump library google.golang.org/grpc to v1.82.0
* Bump indirect libraries dependencies

## 0.0.6
* Build with Go 1.26.4 (stdlib CVE-2026-42504, CVE-2026-42507, CVE-2026-27145) via module `go` version and `GOTOOLCHAIN=go1.26.4` in release builds.

## 0.0.5
* Bump library github.com/inspektor-gadget/inspektor-gadget to v0.53.2
* Bump library google.golang.org/grpc to v1.81.1

## 0.0.4
* Bump library github.com/inspektor-gadget/inspektor-gadget to v0.53.1

## 0.0.3
* Bound the pending event queue with drop-oldest retention (`-max-pending-events` / `MAX_PENDING_EVENTS`, default 50000)
* Send pending events in `BatchSize` chunks instead of draining the full backlog per flush
* Rate-limited warnings when pending events are dropped during sustained backpressure

## 0.0.2
* Add DNS observability via Inspektor Gadget `trace_dns` (DNS_QUERY and DNS_RESPONSE events)

## 0.0.1
* Initial move from insights-ebpf-agent lab repo into insights-plugins as network-flow
