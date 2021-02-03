package fairwinds

blockedNamespace(elem) {
  ns := elem.parameters.blocklist[_]
  elem.metadata.namespace == ns
}

labelblock[actionItem] {
  not blockedNamespace(input)
  provided := {label | input.metadata.labels[label]}
  required := {label | label := input.parameters.labels[_]}
  missing := required - provided
  found := required - missing
  count(found) > 0
  actionItem := {
    "title": "Bad label is present",
    "description": sprintf("Label %v is present", [found]),
    "severity": 0.1,
    "remediation": "Remove the label",
    "category": "Reliability"
  }
}
