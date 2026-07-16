# Workload

Retrieves metadata about running workloads in the current cluster: controllers (and their pods), namespaces, nodes, ingresses, images, and per-namespace object counts.

## Report highlights (2.11+)

* **Nodes** — capacity/allocatable/allocation plus UID, conditions, taints, unschedulable, addresses, provider ID, and nested node info. Top-level `KubeletVersion` and `KubeProxyVersion` remain for Insights compatibility (`KubeProxyVersion` is often empty on modern clusters).
* **Ingresses** — class, rules (hosts/paths/backends), TLS host/secret names, default backend, load-balancer status.
* **NamespaceCounts** — per-namespace counts of pods, services, ingresses, resource quotas, limit ranges, and network policies.

Node addresses, provider IDs, ingress hosts/paths, and TLS secret *names* are included intentionally for inventory; secret data is never read.

## RBAC

In addition to existing workloads list permissions, full `NamespaceCounts` needs cluster `list` on:

* `services`
* `resourcequotas`
* `limitranges`
* `networkpolicies` (`networking.k8s.io`)

If those lists are forbidden, the plugin logs a warning and leaves the corresponding counters at `0` instead of failing the report. Pod and ingress counts still populate from data already fetched for the report.
