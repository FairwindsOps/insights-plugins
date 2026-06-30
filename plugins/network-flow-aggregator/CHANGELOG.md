# Changelog

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
