# Changelog

## 0.0.3
* Bound the pending event queue with drop-oldest retention (`-max-pending-events` / `MAX_PENDING_EVENTS`, default 50000)
* Send pending events in `BatchSize` chunks instead of draining the full backlog per flush
* Rate-limited warnings when pending events are dropped during sustained backpressure

## 0.0.2
* Add DNS observability via Inspektor Gadget `trace_dns` (DNS_QUERY and DNS_RESPONSE events)

## 0.0.1
* Initial move from insights-ebpf-agent lab repo into insights-plugins as network-flow
