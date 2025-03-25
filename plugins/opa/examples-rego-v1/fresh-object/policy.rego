package fairwinds

import rego.v1

blockedNamespace(elem) if {
	# List Kubernetes namespaces where this policy should not be applied.
	blockedNamespaces := ["kube-system"]

	ns := blockedNamespaces[_]
	elem.metadata.namespace == ns
}

objectfresh contains actionItem if {
	# Define the number of days, after which objects are considered
	# to be stale.
	staleDays := 10

	# List the Kubernetes Kinds to which this policy should apply.
	kinds := {"Deployment", "DaemonSet", "StatefulSet", "CronJob", "Job"}

	not blockedNamespace(input)
	kind := lower(kinds[val])
	lower(input.kind) == kind
	comparisonDate := time.add_date(time.now_ns(), 0, 0, 0 - staleDays)
	input.metadata.creationTimestamp < comparisonDate
	description := sprintf("Creation time %v is too old", [input.metadata.creationTimestamp])
	actionItem := {
		"title": "Object is stale",
		"description": description,
		"severity": 0.1,
		"remediation": "Consider updating or deleting this object",
		"category": "Reliability",
	}
}
