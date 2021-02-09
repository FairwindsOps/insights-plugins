package fairwinds

blockedNamespace(elem) {
  ns := elem.parameters.blocklist[_]
  elem.metadata.namespace == ns
}

annotationrequired[actionItem] {
  not blockedNamespace(input)
  provided := {annotation | input.metadata.annotations[annotation]}
  required := {annotation | annotation := input.parameters.annotations[_]}
  missing := required - provided
  count(missing) > 0
  actionItem := {
    "title": "Annotation is missing",
    "description": sprintf("Annotation %v is missing", [missing]),
    "severity": 0.1,
    "remediation": "Add the annotation",
    "category": "Reliability"
  }
}
