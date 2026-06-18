# network-flow-aggregator To-Dos

## Upstream pending queue

The local `store` is bounded (`-max-events`, `-max-age`), but `upstream.Client.pending` grows without limit during Insights outages. Pending batches hold pointers to enriched events, so memory can grow even after the store evicts old rows.

- [ ] Cap pending by total event count (drop-oldest batches on enqueue), reusing `-max-events` / `-max-age`
- [ ] Log or metric dropped pending events during sustained backpressure
- [ ] Fix `sendPending`: on mid-flush failure, requeue unsent batches (today only the failed batch is requeued; later batches in the same flush are lost)
- [ ] Longer term: single buffer with a send cursor from `store` instead of a duplicate pending queue
