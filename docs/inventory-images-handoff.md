# Handoff: Inventory Images — workloads plugin (Phase 1)

**Repo:** `insights-plugins`  
**Plugin:** `plugins/workloads`  
**Target version:** `2.10.0`  
**Design (Insights):** `/Users/james/git/Insights/docs/inventory-images.md`  
**Full plan:** `/Users/james/git/Insights/docs/inventory-images-implementation-plan.md`

Use this doc as the sole context to implement Phase 1. Insights ingest/API is a separate repo/PR.

---

## Goal

Add top-level **`Images[]`** to the workloads JSON report: every **running** container image in the cluster (regular, init, ephemeral) with **workload owners**.

Do **not** use `Controllers[].Containers[].Image` / `ImageID` for inventory — those come from pod **spec** and `ImageID` is empty today.

---

## Report shape (add to `ClusterWorkloadReport`)

```json
{
  "ServerVersion": "1.32",
  "CreationTime": "2026-06-08T12:00:00Z",
  "Controllers": ["..."],
  "Nodes": ["..."],
  "Images": [
    {
      "Name": "quay.io/fairwinds/goldilocks:v2.2.0",
      "ID": "quay.io/fairwinds/goldilocks@sha256:abc123...",
      "PullRef": "quay.io/fairwinds/goldilocks@sha256:abc123...",
      "Owners": [
        {
          "Namespace": "insights-agent",
          "Kind": "Deployment",
          "Name": "insights-agent-goldilocks-controller",
          "Container": "goldilocks"
        }
      ]
    }
  ]
}
```

| Field | Source |
|-------|--------|
| `Name` | `containerStatus.Image`; if `sha256…` prefix, fall back to `ImageID` |
| `ID` | `containerStatus.ImageID` minus `docker-pullable://` |
| `PullRef` | `ID` if pullable, else `Name` |
| `Owners` | Top controller, or `Kind: Pod` / `Kind: Job` for supplemental sweeps |

**Dedupe key:** `Name + "/" + ID`. Multiple owners per image.

**Skip** entries where `ID` is empty after normalization (log warning). Insights will not ingest tag-only rows.

---

## Discovery: copy/adapt image-trust sweep

**Reference:** `plugins/image-trust/pkg/discovery/images.go`

Create **`plugins/workloads/pkg/discovery/`** with `ListRunningImages(ctx, kubeClient) (Result, error)`.

### Three-pass sweep (full cluster — no namespace allow/block)

| Pass | Logic | Owner `Kind` |
|------|--------|--------------|
| 1 | `GetAllTopControllersWithPods("")` → pods → statuses | Deployment, StatefulSet, etc. |
| 2 | All namespaces → `pods` with `status.phase=Running` not seen in pass 1, no `controller=true` owner | `Pod` |
| 3 | All namespaces → `jobs` with `status.active > 0` → Running pods by selector, not seen | `Job` |

### Differences from image-trust (inventory-specific)

| image-trust | workloads inventory |
|-------------|---------------------|
| Namespace allow/block env | **None** — all namespaces |
| Pass 1: all pod phases | **Running pods only** on all passes |
| `index.docker.io/` prefix | Use same normalization as image-trust (`docker-pullable://` strip, docker prefix) |

### Container statuses (all passes)

From `containerStatusesFromPod`:

- `status.containerStatuses`
- `status.initContainerStatuses`
- `status.ephemeralContainerStatuses`

### RBAC

Workloads SA binds cluster `view` — sufficient for `pods` and `jobs` list (passes 2–3).

---

## Files to change

| File | Action |
|------|--------|
| `plugins/workloads/pkg/discovery/images.go` | **New** — `ListRunningImages`, sweep helpers |
| `plugins/workloads/pkg/discovery/result.go` | **New** — `Result`, `ImageResult`, `OwnerResult` types |
| `plugins/workloads/pkg/discovery/images_test.go` | **New** — unit tests |
| `plugins/workloads/pkg/info.go` | Add `Images []ImageResult` to report; call discovery |
| `plugins/workloads/cmd/main.go` | Already passes `kube` — wire discovery in `CreateResourceProviderFromAPI` |
| `plugins/workloads/results.schema` | Add `Images` array |
| `plugins/workloads/version.txt` | `2.10.0` |
| `plugins/workloads/CHANGELOG.md` | Entry for `Images[]` |

### `info.go` integration sketch

`CreateResourceProviderFromAPI` already calls `GetAllTopControllersWithPods`. Either:

- Call `discovery.ListRunningImages(ctx, kube)` as a separate pass (simplest; duplicates controller list), or
- Refactor to pass the `workloads` slice from the existing loop into discovery (one fewer API walk).

Prefer correctness first; optimize duplicate `GetAllTopControllersWithPods` later if needed.

Map discovery types → exported JSON types on `ClusterWorkloadReport`:

```go
type ImageOwnerResult struct {
    Namespace string
    Kind      string
    Name      string
    Container string
}

type ImageResult struct {
    Name    string
    ID      string
    PullRef string
    Owners  []ImageOwnerResult
}
```

---

## Tests (`pkg/discovery/images_test.go`)

No live cluster required. Cover:

- [ ] Dedupe: same `Name`+`ID`, two owners attached
- [ ] `docker-pullable://` stripped from `ID`
- [ ] Init container status included
- [ ] Empty `ImageID` → image not added (or skipped with warn)
- [ ] `hasControllerOwner` / orphan pod → owner `Kind: Pod`
- [ ] `containerStatusesFromPod` merges all three status slices

Follow patterns in `plugins/image-trust/pkg/discovery/images_test.go` and `plugins/workloads/pkg/actual_test.go`.

---

## Acceptance criteria

- [ ] `workload --output-file /tmp/w.json` on a dev cluster includes non-empty `Images`
- [ ] Every `Images[].ID` is non-empty
- [ ] Owners include Deployment/StatefulSet controllers and match kubectl running images
- [ ] `go test ./plugins/workloads/...` passes
- [ ] `results.schema` validates sample output
- [ ] Version `2.10.0` + CHANGELOG

---

## Out of scope (this PR)

- Insights ingest / DB / API
- Namespace env vars
- Registry resolve/verify (image-trust `resolve` / `verify`)
- Changing `Controllers[].Containers` spec image behavior
- Extracting shared discovery to `controller-utils`

---

## Suggested PR description

```
workloads: add Images[] to report for cluster inventory (v2.10.0)

Runtime image discovery via image-trust-style sweep (controllers,
orphan pods, active jobs). Running pods only; init/ephemeral included.

Insights will ingest Images[] in a follow-up PR.
```

---

## Prompt for new Cursor tab

Open **`insights-plugins`** repo → new chat → attach this file:

> Implement Phase 1 from `docs/inventory-images-handoff.md`: add `plugins/workloads/pkg/discovery`, wire `Images[]` into `ClusterWorkloadReport`, update schema and version to 2.10.0, add tests. Follow image-trust discovery; full cluster scope; Running-only on all passes; skip empty `ID`.
