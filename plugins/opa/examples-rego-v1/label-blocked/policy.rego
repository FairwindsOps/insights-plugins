package fairwinds

import rego.v1

blockedNamespace(elem) if {
	# List Kubernetes namespaces where this policy should not be applied.
	blockedNamespaces := ["kube-system"]

	ns := blockedNamespaces[_]
	elem.metadata.namespace == ns
}

labelblock contains actionItem if {
	# List the keys of Kubernetes labels that will be blocked.
	blockedLabels := {"foo"}

	# List the Kubernetes Kinds to which this policy should apply.
	kinds := {"Deployment", "DaemonSet", "StatefulSet", "CronJob", "Job"}

	not blockedNamespace(input)
	kind := lower(kinds[val])
	lower(input.kind) == kind
	provided := {label | input.metadata.labels[label]}
	missing := blockedLabels - provided
	found := blockedLabels - missing
	count(found) > 0
	actionItem := {
		"title": "Bad label is present",
		"description": sprintf("Label %v is present", [found]),
		"severity": 0.1,
		"remediation": "Remove the label",
		"category": "Reliability",
	}
}
