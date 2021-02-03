package fairwinds

notinnamespace[actionItem] {
  namespace := input.parameters.namespaces[_]
  input.metadata.namespace == namespace
  description := sprintf("Namespace %v is forbidden", [namespace])
  actionItem := {
    "description": description,
    "title": "Creating resources in this namespace is forbidden",
    "severity": 0.1,
    "remediation": "Move this resource to a different namespace",
    "category": "Reliability"
  }
}
