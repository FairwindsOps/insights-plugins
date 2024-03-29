package fairwinds
blockedNamespace(elem) {
    ns := elem.parameters.blocklist[_]
    elem.metadata.namespace == ns
}
latestReplicaDateStale(replicaSets, elem) {
    rs := replicaSets[_]
    owner := rs.metadata.ownerReferences[_]
    owner.uid == elem.metadata.uid
    rs.status.replicas > 0
    comparisonDate := time.add_date(time.now_ns(), 0, 0, 0 - input.parameters.days)
    time.parse_rfc3339_ns(rs.metadata.creationTimestamp) < comparisonDate
}
freshDeployment[actionItem] {
    not blockedNamespace(input)
    latestReplicaDateStale(kubernetes("apps", "ReplicaSet"), input)
    actionItem := {
        "title": "Deployment is out of date",
        "description": "No fresh replica sets found",
        "severity": 0.1,
        "remediation": "Update this deployment",
        "category": "Reliability"
    }
}
