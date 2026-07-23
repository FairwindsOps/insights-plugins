# Workload

Retrieves metadata about running workloads in the current cluster: controllers (and their pods), namespaces, nodes, ingresses, services, persistent volume claims, images, Karpenter CRDs (when present), and per-namespace object counts.

## Report highlights (2.15+)

* **Karpenter inventory** — top-level `NodePools[]`, `NodeClaims[]`, and `EC2NodeClasses[]` (AWS only; Azure/GCP NodeClasses out of scope). Always emitted, including empty arrays. Listed via the dynamic client with explicit `karpenter.sh/v1` / `karpenter.k8s.aws/v1` GVRs; soft-fail (warn + empty arrays) when CRDs are not installed or list is forbidden. Agent ClusterRoles need companion `list` rules for these resources or the arrays stay empty in production.

## Report highlights (2.14+)

* **Controller NodeNames** — each controller may include `NodeNames[]` listing unique nodes where its Running pods are scheduled.

## Report highlights (2.13+)

* **Controller VolumeClaims** — each controller may include `VolumeClaims[]` with pod volume name and PVC `claimName` (from the template and running pods).

## Report highlights (2.12+)

* **Services** — type, cluster IPs, selector, ports, external name/IPs, and load-balancer status.
* **PersistentVolumeClaims** — storage class, access modes, volume mode/name, request/capacity storage, and phase.

## Report highlights (2.11+)

* **Nodes** — capacity/allocatable/allocation plus UID, conditions, taints, unschedulable, addresses, provider ID, and nested node info. Top-level `KubeletVersion` and `KubeProxyVersion` remain for Insights compatibility (`KubeProxyVersion` is often empty on modern clusters).
* **Ingresses** — class, rules (hosts/paths/backends), TLS host/secret names, default backend, load-balancer status.
* **NamespaceCounts** — per-namespace counts of pods, services, ingresses, resource quotas, limit ranges, and network policies.

Node addresses, provider IDs, ingress hosts/paths, TLS secret *names*, service selectors/ports, and PVC names/sizes are included intentionally for inventory; secret data is never read.

## RBAC

In addition to existing workloads list permissions, inventory and full `NamespaceCounts` need cluster `list` on:

* `services` (required for `Services[]` and `NamespaceCounts.ServiceCount`)
* `persistentvolumeclaims` (required for `PersistentVolumeClaims[]`)
* `resourcequotas`
* `limitranges`
* `networkpolicies` (`networking.k8s.io`)
* `nodepools`, `nodeclaims` (`karpenter.sh`) — optional; missing list leaves Karpenter arrays present but empty
* `ec2nodeclasses` (`karpenter.k8s.aws`) — optional; missing list leaves `EC2NodeClasses` present but empty

If ResourceQuota / LimitRange / NetworkPolicy / Karpenter CRD lists are forbidden or CRDs are absent, the plugin logs a warning and leaves the corresponding fields empty/`0` instead of failing the report. Missing Service or PVC list permission fails the report (same as Ingress). Pod and ingress counts still populate from data already fetched for the report.
