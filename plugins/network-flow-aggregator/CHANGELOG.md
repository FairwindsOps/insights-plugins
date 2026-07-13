# Changelog

## 0.0.8
* Enriches existing client→Service flow events with optional backend workload/pod identity using EndpointSlice pod lookup and server-side peer correlation. This enables Service→backend attribution on Insights.

## 0.0.7
* Bump library google.golang.org/grpc to v1.82.0
* Bump indirect libraries dependencies

## 0.0.6
* Add TLS support for Insights upstream gRPC client (auto-enabled on port 443)

## 0.0.5
* Build with Go 1.26.4 (stdlib CVE-2026-42504, CVE-2026-42507, CVE-2026-27145) via module `go` version and `GOTOOLCHAIN=go1.26.4` in release builds.

## 0.0.4
* Bump library google.golang.org/grpc to v1.81.1

## 0.0.3
* Replace upstream pending queue with store send cursor so retention limits apply to unsent events
* Fix upstream flush to retain unsent events on mid-stream send failure
* Log rate-limited warnings when retention drops unsent events during backpressure

## 0.0.2
* Add DNS IP-to-hostname cache and `ExternalHostname` destination enrichment for service maps

## 0.0.1
* Initial move from insights-ebpf-agent lab repo into insights-plugins as network-flow-aggregator
