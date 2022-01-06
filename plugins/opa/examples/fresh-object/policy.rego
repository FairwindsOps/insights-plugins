package fairwinds
blockedNamespace(elem) {
    ns := elem.parameters.blocklist[_]
    elem.metadata.namespace == ns
}
objectfresh[actionItem] {
    not blockedNamespace(input)
    comparisonDate := time.add_date(time.now_ns(), 0, 0, 0 - input.parameters.days)
    input.metadata.creationTimestamp < comparisonDate
    description := sprintf("Creation time %v is too old", [input.metadata.creationTimestamp])
    actionItem := {
        "title": "Object is stale",
        "description": description,
        "severity": 0.1,
        "remediation": "Consider updating or deleting this object",
        "category": "Reliability"
    }
}
