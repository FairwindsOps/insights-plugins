package fairwinds
blockedNamespace(elem) {
  ns := elem.parameters.blocklist[_]
  elem.metadata.namespace == ns
}
chartfresh[actionItem] {
  not blockedNamespace(input)
  comparisonDate := time.add_date(time.now_ns(), 0, 0, 0 - input.parameters.days)
  startswith(input.metadata.name, "sh.helm.release.v1")
  input.metadata.labels.owner == "helm"
  input.metadata.labels.status == "deployed"
  time.parse_rfc3339_ns(input.metadata.creationTimestamp) < comparisonDate
  description := sprintf("Creation time %v is too old", [input.metadata.creationTimestamp])
  actionItem := {
    "title": "Helm chart is stale",
    "description": description,
    "severity": 0.1,
    "remediation": "Consider updating or deleting this chart",
    "category": "Reliability"
  }
}
