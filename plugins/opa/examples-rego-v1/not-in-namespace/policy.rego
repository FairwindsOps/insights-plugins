package fairwinds

import rego.v1

notinnamespace contains actionItem if {
	# List Kubernetes namespaces which are forbidden.
	blockedNamespaces := ["default"]

	namespace := blockedNamespaces[_]
	input.kind == "Pod"
	input.metadata.namespace == namespace
	description := sprintf("Namespace %v is forbidden", [namespace])
	actionItem := {
		"description": description,
		"title": "Creating resources in this namespace is forbidden",
		"severity": 0.1,
		"remediation": "Move this resource to a different namespace",
		"category": "Reliability",
	}
}
