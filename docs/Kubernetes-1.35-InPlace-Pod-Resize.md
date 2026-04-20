# Kubernetes 1.35 In-Place Pod Resize: Impact and Plan

**Last updated:** April 2026  
**Scope:** What changes with in-place resize, what it means for Insights, and a practical implementation plan.

---

## Summary

Kubernetes 1.35 makes in-place pod resize GA: CPU and memory requests/limits are mutable via the pod `resize` subresource.

- **Desired:** `spec.containers[*].resources`
- **Actual (applied):** `status.containerStatuses[*].resources`
- **Scheduler/internal tracking:** `status.containerStatuses[*].allocatedResources` and resize conditions

For Insights, the current behavior is still valid, but not always current for clusters using in-place resize:

- `insights-plugins` workloads currently reports **spec/template** resources.
- Insights backend stores those values in `k8s_resources.container_resources`.
- Cost logic uses billed as `max(usage, request)` from these reported values.

Result: if resources are changed in-place without corresponding template updates, UI/cost/recommendation "current" values may lag real runtime state.

---

## What changed in Kubernetes 1.35

In-place pod resize is stable and enabled by default in Kubernetes 1.35.

- CPU and memory requests/limits can be changed without replacing the pod.
- Updates are performed through the pod `resize` subresource.
- `status.containerStatuses[*].resources` reflects currently applied resources.
- Pod conditions (`PodResizePending`, `PodResizeInProgress`) describe resize lifecycle and failures.

Important caveat: in-place does not guarantee "no restart." Actual restart behavior depends on container `resizePolicy` and runtime support.

---

## Current behavior in our codebases

### `insights-plugins`

- **Workloads plugin** uses `GetAllTopControllersWithPods` and emits **spec/template** CPU and memory in `Resource.Requests`/`Resource.Limits`; when enough **Running + Ready** pods agree, it also emits optional **`Resource.Actual`** from `status.containerStatuses[].resources` (per-container majority; omitted if ambiguous).
- **Node allocation** uses `PodRequestsAndLimits(pod)` from `pod.Spec`; this should remain spec-based.
- **Prometheus plugin** queries `kube_pod_container_resource_requests/limits` (spec-oriented metrics).
- **Right-sizer plugin** can patch controller memory limits, but there is no current in-place resize implementation in code/TODO docs.

### `~/git/Insights` backend

- Workloads report parsing and persistence is in `pkg/reports/workloads.go`.
- Container spec values are stored in `k8s_resources.container_resources` (`CPURequestsMillis`, `MemoryRequestsBytes`, etc.).
- Cost calculation path (`pkg/database/metrics.go`) computes billed from usage vs request and applies cloud rates.
- Node capacity history is fed by node allocated request/limit totals and is separate from workload billed-cost math.

---

## Impact assessment

| Area | Impact | Why |
|---|---|---|
| Right-sizer | High opportunity | Can reduce disruption by attempting in-place resize after updating controller spec. |
| Workloads report | Medium | Currently spec/template only; may lag actual when pods are resized in-place. |
| Cost and recommendations | Medium | Billed math is right, but input request values can be stale if only template/spec is reported. |
| Node allocation/capacity | Keep as-is | Should remain spec/scheduling semantics, not runtime-applied values. |

---

## Corrections and clarifications

The previous draft was directionally correct but had a few issues:

1. **Overstated VPA wording**
   - Safer wording: in `InPlaceOrRecreate`, VPA updater attempts in-place first and can fall back to eviction/recreate when needed.

2. **Right-sizer TODO link mismatch**
   - `plugins/right-sizer/TODO.md` currently does not track in-place resize work.
   - Either add a TODO item there or remove that internal pointer.

3. **"First pod with resources wins" is risky**
   - During rollouts, replicas can differ; selecting the first pod with status resources can produce noisy or misleading "actual."
   - Use a deterministic policy (recommended below).

4. **Prometheus caveat**
   - kube-state-metrics has active work to expose status-based resource metrics.
   - We should treat provider/version differences explicitly rather than assume one universal metric shape.

---

## Recommended plan (prioritized)

### Now (docs/product)

- Document that workload resource views and costs are currently based on desired/spec inputs.
- Note that in-place resize may not be fully reflected until template-based data catches up (or actual support is enabled).

### Next (plugin implementation)

- Add optional actual resources to workloads output.
- Keep existing spec fields unchanged for backward compatibility.
- Keep node allocation spec-based.

### Later (backend + product behavior)

- Persist optional actual values in backend.
- Add a setting: **Use actual resources for workload cost when available** (default off).
- Update UI labels to clearly distinguish:
  - **Configured (desired/spec)**
  - **Applied (actual/status)**

---

## Implementation details

### Phase 1: `insights-plugins` workloads

**Status:** Implemented in this repo (workloads plugin).

1. Switch from `GetAllTopControllersSummary` to `GetAllTopControllersWithPods` in workloads collection.
2. Add optional `Actual` under each container resource in report model and `plugins/workloads/results.schema`.
3. Convert pods from unstructured to `corev1.Pod`, then read:
   - `pod.Status.ContainerStatuses[*].Resources.Requests`
   - `pod.Status.ContainerStatuses[*].Resources.Limits`
4. Populate `Actual` with a deterministic selection policy:
   - consider only **Running** pods with **PodReady=True**;
   - per container name, group pods by applied request/limit fingerprint; **majority wins**;
   - if there is a tie for the highest vote count (including 1:1 splits during rollouts), **omit** `Actual` for that container.
5. Keep existing `Requests`/`Limits` fields mapped from spec unchanged.
6. Keep node allocation logic unchanged (`PodRequestsAndLimits(pod.Spec)` semantics).
7. Add tests for:
   - no status resources -> unchanged output,
   - status resources present -> `Actual` emitted,
   - mixed replicas during rollout -> deterministic behavior.

### Phase 2: `~/git/Insights` backend

1. Extend workloads report parsing (`pkg/reports/workloads.go`) to accept optional container `Actual`.
2. Persist actual values separately from existing spec columns (do not overwrite legacy request/limit fields).
3. Update cost pipeline (`pkg/database/metrics.go`) behind a setting:
   - default: existing spec-based behavior,
   - optional: use actual request for billed comparisons when available.
4. Add migration and API/query support to expose both desired and actual consistently.
5. Add regression tests for:
   - legacy reports (no actual),
   - mixed availability of actual by container,
   - toggling cost setting on/off.

### Phase 3: optional improvements

- Align Prometheus ingestion if/when status-resource metrics are available in target KSM versions.
- Update Goldilocks/right-sizing UI semantics so "current" can reflect applied values.

---

## Prometheus, kube-state-metrics, and kubelet (deeper dive)

### What today’s stack usually exposes

| Signal | Typical source | Tracks **desired** or **applied**? |
|--------|----------------|--------------------------------------|
| `kube_pod_container_resource_requests` / `kube_pod_container_resource_limits` | kube-state-metrics (KSM) | **Desired** — derived from pod **spec** (same family our plugin queries). |
| `container_memory_usage_bytes`, `rate(container_cpu_usage_seconds_total[…])` | cAdvisor / kubelet | **Usage** (observed), not request/limit. |
| Pod object in API | `status.containerStatuses[].resources` | **Applied** (what the kubelet/runtime reports). |

During in-place resize, **spec** can move ahead of **applied** (e.g. resize **Deferred** / **Infeasible** until the kubelet applies it). KSM’s existing resource metrics still follow spec, so Prometheus-only “request/limit” panels can disagree with `kubectl get pod -o yaml` status until things converge.

### kube-state-metrics: gap and in-flight work

Upstream issue (accepted): [Expose actual pod CPU/memory request from `status.containerStatuses.resources` (#2665)](https://github.com/kubernetes/kube-state-metrics/issues/2665) — motivates metrics from **status**, tied to [KEP-1287](https://github.com/kubernetes/enhancements/issues/1287).

Competing / overlapping PRs (both were open and stalled as of early 2026; confirm before coding to a name):

- [PR #2702](https://github.com/kubernetes/kube-state-metrics/pull/2702): adds `kube_pod_container_actual_resource_requests` and `kube_pod_container_actual_resource_limits` from status.
- [PR #2773](https://github.com/kubernetes/kube-state-metrics/pull/2773): adds `kube_pod_container_status_resource_requests`, `kube_pod_container_status_resource_limits`, plus init-container variants (naming aligned with other `kube_pod_container_status_*` metrics).

Maintainers discussed consolidating naming and sharing parsing logic; **final metric names are not stable until one PR merges**.

Vendor precedent: some agents prefer status over spec for CPU/memory during in-place vertical scaling — e.g. [newrelic/nri-kubernetes#1433](https://github.com/newrelic/nri-kubernetes/pull/1433).

### Kubelet metrics (resize operations)

Kubernetes documents **ALPHA** kubelet metrics useful for ops (pending/infeasible resizes, duration), not a full substitute for per-container “applied request” in PromQL:

- `kubelet_container_requested_resizes_total`
- `kubelet_pod_pending_resizes`, `kubelet_pod_in_progress_resizes`
- `kubelet_pod_infeasible_resizes_total`, `kubelet_pod_deferred_accepted_resizes_total`
- `kubelet_pod_resize_duration_milliseconds`

See the authoritative list: [Kubernetes metrics reference (kubelet)](https://kubernetes.io/docs/reference/instrumentation/metrics/) (search for `resize` on that page).

Historical note: work also targeted a `kubelet_resize_requests_total`-style counter ([kubernetes/kubernetes#128111](https://github.com/kubernetes/kubernetes/pull/128111)); rely on the **current** documented metric names for your cluster version.

### Fairwinds `prometheus` plugin (this repo)

`plugins/prometheus/pkg/data/prometheus.go` hard-codes:

- `kube_pod_container_resource_requests` / `kube_pod_container_resource_limits` (with `unit=` filters for CPU/memory),
- plus GMP/GKE fallbacks (`kubernetes_io:container_*`) when KSM returns **no time series** (`len(matrix)==0`), not when KSM values are “wrong” vs applied resources.

There is **no** query path today for status-derived request/limit series. Any future support should:

1. Prefer **status** metrics when present (after verifying exact names for the customer’s KSM version), else fall back to spec metrics.
2. Use `or` / `label_replace` carefully so cardinality and label sets stay join-compatible with existing usage queries.

### Google Cloud (GKE / Managed Service for Prometheus)

**How we detect GMP**

- `plugins/prometheus/pkg/data/data.go` — `IsGMP(prometheusAddress)` is true when the host is `monitoring.googleapis.com` (Managed Service for Prometheus / same API as Cloud Monitoring).
- `plugins/prometheus/cmd/prometheus-collector/main.go` — uses Application Default Credentials when the address is Google’s monitoring endpoint (see `plugins/prometheus/README.md` for example `PROMETHEUS_ADDRESS`).

**How request/limit collection differs from non-GCP**

- **Primary path is unchanged:** we still query kube-state-metrics-style metrics first (`kube_pod_container_resource_*` with `unit=` filters).
- **Fallback path (GMP only):** for **each** of the CPU/memory request/limit queries, if that query’s result matrix is **empty** (`len(matrix)==0`) and `clusterName` is set, we retry **that metric only** using **GKE system metrics** exposed in PromQL as:

  `kubernetes_io:container_<name>{monitored_resource="k8s_container", cluster_name="<cluster>"}`

  Implemented in `queryGKEContainerMetric` — metrics used today:

  | Fallback metric (Prometheus form) | Cloud Monitoring type (docs) |
  |-----------------------------------|-------------------------------|
  | `kubernetes_io:container_memory_request_bytes` | `kubernetes.io/container/memory/request_bytes` |
  | `kubernetes_io:container_memory_limit_bytes` | `kubernetes.io/container/memory/limit_bytes` |
  | `kubernetes_io:container_cpu_request_cores` | `kubernetes.io/container/cpu/request_cores` |
  | `kubernetes_io:container_cpu_limit_cores` | `kubernetes.io/container/cpu/limit_cores` |
  | GPU: `kubernetes_io:container_accelerator_request` (summed by container) | `kubernetes.io/container/accelerator/request` |

- **Label normalization:** GKE often emits `namespace_name`, `pod_name`, `container_name`; `normalizeGKEContainerMatrix` maps them to `namespace`, `pod`, `container` so the rest of the pipeline matches KSM joins.
- **GPU limits on GKE:** there is no separate limit series in GKE system metrics — code uses **accelerator request** as a best-effort stand-in for both request and limit when KSM has no GPU limit data (`getGPULimitsGKE`).

**Semantics vs in-place resize (GCP)**

- Google’s metric catalog describes these as the container’s **memory request/limit** and **CPU request/limit** (gauges on the `k8s_container` monitored resource); it does **not** use the same “desired spec only” wording as kube-state-metrics. In practice they are **GKE-collected system metrics** and may track kubelet/runtime-visible configuration more closely than raw API spec in some cases — but **Google does not clearly document spec vs `status.containerStatuses.resources` for every resize edge case** (e.g. **Deferred** / **Infeasible** with desired spec already patched).
- **Implication:** do not assume the GKE fallback automatically fixes IPR skew. It only runs when KSM returns **no data** for that metric family; if KSM is present and scraping spec, we **never** switch to `kubernetes_io:container_*` for that dimension. For authoritative **applied** resources, the API field `status.containerStatuses[].resources` (or future KSM status metrics) remains the reference until validated otherwise on your GKE version.

**Fact-check: common GMP / KSM claims about “actual” limits**

You may see guidance that conflates **desired (spec)** with **applied (status)**. For in-place resize, that matters when resize is **Deferred** or **Infeasible**: the API **spec** can already show the new request/limit while **`status.containerStatuses[].resources`** still reflects the old cgroup until the kubelet applies (or forever if infeasible).

| Claim | Safer reading |
|--------|----------------|
| “`kube_pod_container_resource_limits` is the **current limit applied** and updates when the kubelet applies the resize.” | **Usually incorrect as stated.** In standard kube-state-metrics, `kube_pod_container_resource_*` series are built from the pod **spec** (desired). They track the same field you patch via the `resize` subresource on the spec side — not a separate “runtime-only” gauge. When spec and status diverge, KSM’s usual resource metrics follow **spec**, not `status.containerStatuses[].resources`. |
| “`kubernetes.io/container/cpu/limit_cores` (GMP: `kubernetes_io:container_cpu_limit_cores`) is the **actual** limit vs spec.” | **Partially useful, not formally equivalent.** GKE documents these as the container’s CPU/memory limit (and request) on `k8s_container`. They may align with what the node/runtime is enforcing **in many steady states**, but Google does not publish a guarantee that they always equal `status.containerStatuses[].resources` in every resize edge case. **Validate with `kubectl get pod -o yaml`** on a test workload if you rely on this for compliance or cost. |
| “KSM v2.10+ exposes `kube_pod_container_status_allocated_resources_*` for `allocatedResources`.” | **Do not rely on this without checking your binary.** As of early 2026, exposing **status** resources / allocated fields is still **under active upstream work** (e.g. [KSM #2665](https://github.com/kubernetes/kube-state-metrics/issues/2665), [PR #2702](https://github.com/kubernetes/kube-state-metrics/pull/2702), [PR #2773](https://github.com/kubernetes/kube-state-metrics/pull/2773)). There is **no** widely shipped, stable metric that is a drop-in `kube_pod_container_actual_resource_limits` for all clusters. |

**Practical comparison in GMP (when you have both sources)**

- You **can** plot **KSM** `kube_pod_container_resource_limits` **and** **GKE** `kubernetes_io:container_cpu_limit_cores` / `kubernetes_io:container_memory_limit_bytes` (same `cluster_name`, align `namespace`/`pod`/`container` labels) to spot **divergence** — but interpret divergence as “different pipelines / semantics / lag”, not automatically “spec wrong, GKE right” without ground truth from the API.
- **Resize conditions** (`PodResizePending`, `PodResizeInProgress`, reasons `Deferred` / `Infeasible`) are on the **Pod object**, not on container metrics. Monitoring them may require **kube-state-metrics pod condition metrics** (if enabled in your deployment), **kubelet** resize metrics (node-local), or **events** — not a single `kube_pod_container_*` series.

**Optional direction: PromQL for “something is off”**

- A full **spec-vs-applied** PromQL join is **not** standard until stable status-resource series exist in KSM (or you export custom metrics from the API).
- Interim signals: kubelet counters/gauges for resize backlog ([kubelet metrics](https://kubernetes.io/docs/reference/instrumentation/metrics/) — search `resize`), plus manual or scripted checks of pod YAML when alerts fire.

**References (GCP)**

- [GKE system metrics (Kubernetes metrics)](https://cloud.google.com/monitoring/api/metrics_kubernetes) — search for `container/memory/request_bytes`, `container/cpu/request_cores`, etc.
- [Monitored resource: `k8s_container`](https://cloud.google.com/monitoring/api/resources#tag_k8s_container) — `cluster_name` and container identity labels.
- In-repo: `plugins/prometheus/pkg/data/data.go` (`GetMetrics`, `useGKEFallback`), `plugins/prometheus/pkg/data/prometheus.go` (`queryGKEContainerMetric`, `gkeSystemMetricsPrefix`).

### Practical PromQL / dashboard implications

- **Utilization vs limit**: `usage / limit` where `limit` comes from KSM spec can be wrong transiently if spec limit was raised but the container is still on the old cgroup limit — treat as a known edge case or join to status-based limits when available.
- **Billing / cost alignment**: Insights already does `max(usage, request)` in the backend; the **request** side often comes from workloads + these KSM series — stale **desired** request in Prometheus produces the same class of skew as a stale controller template in the workloads report.

---

## Open questions

- **Resolved for Phase 1 workloads:** when replicas disagree on applied resources, use a **majority** among Running+Ready pods; **omit `Actual`** when the top vote is tied or there is no data.
- Should actual-resource cost mode be cluster-level, org-level, or feature-flagged first?
- Do we want a rollout gate (for example, only show actual when at least N% of ready pods agree)?

---

## References

- Kubernetes blog: [Kubernetes 1.35: In-Place Pod Resize Graduates to Stable](https://kubernetes.io/blog/2025/12/19/kubernetes-v1-35-in-place-pod-resize-ga/)
- Kubernetes docs: [Resize CPU and Memory Resources assigned to Containers](https://kubernetes.io/docs/tasks/configure-pod-container/resize-container-resources/)
- VPA AEP: [AEP-4016 In-Place Updates Support](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler/enhancements/4016-in-place-updates-support)
- Google Cloud: [GKE / Kubernetes metrics (Cloud Monitoring)](https://cloud.google.com/monitoring/api/metrics_kubernetes)
- Internal code paths:
  - `insights-plugins/plugins/workloads/pkg/info.go`
  - `insights-plugins/plugins/workloads/pkg/actual.go` (aggregates optional `Actual`)
  - `insights-plugins/plugins/workloads/pkg/allocated.go`
  - `insights-plugins/plugins/prometheus/pkg/data/prometheus.go`
  - `insights-plugins/plugins/prometheus/pkg/data/data.go` (GMP detection + GKE fallback)
  - `insights-plugins/plugins/prometheus/README.md` (GMP address / auth)
  - `Insights/pkg/reports/workloads.go`
  - `Insights/pkg/database/metrics.go`
