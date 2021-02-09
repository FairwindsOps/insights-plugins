package fairwinds

blockedNamespace(elem) {
  ns := elem.parameters.blocklist[_]
  elem.metadata.namespace == ns
}

labelrequired[actionItem] {
  not blockedNamespace(input)
  provided := {label | input.metadata.labels[label]}
  required := {label | label := input.parameters.labels[_]}
  missing := required - provided
  count(missing) > 0
  description := sprintf("Label %v is missing", [missing])
  severity := 0.1 * count(missing)
  actionItem := {
    "title": "Label is missing",
    "description": description,
    "severity": severity,
    "severity": 0.1,
    "remediation": "Add the label",
    "category": "Reliability"
  }
}
