package fairwinds
blockedNamespace(elem) {
    # List Kubernetes namespaces where this policy should not be applied.
    blockedNamespaces := ["kube-system"]

    ns := blockedNamespaces[_]
    elem.metadata.namespace == ns
}
latestReplicaDateStale(replicaSets, elem) {
    # Define the number of days, after which replicas are considered
    # to be stale.
    staleDays := 60
    rs := replicaSets[_]
    owner := rs.metadata.ownerReferences[_]
    owner.uid == elem.metadata.uid
    rs.status.replicas > 0
    comparisonDate := time.add_date(time.now_ns(), 0, 0, 0 - staleDays)
    time.parse_rfc3339_ns(rs.metadata.creationTimestamp) < comparisonDate
}
freshDeployment[actionItem] {
    not blockedNamespace(input)
input.kind == "Deployment"
    latestReplicaDateStale(kubernetes("apps", "ReplicaSet"), input)
    actionItem := {
        "title": "Deployment is out of date",
        "description": "No fresh replica sets found",
        "severity": 0.1,
        "remediation": "Update this deployment",
        "category": "Reliability"
    }
}
