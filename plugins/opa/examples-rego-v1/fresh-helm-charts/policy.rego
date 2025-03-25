package fairwinds

import rego.v1

# Note that this policy will not function without updating the OPA plugin
# target-resource and RBAC configuration to include Secret resources.

blockedNamespace(elem) if {
	# List Kubernetes namespaces where this policy should not be applied.
	blockedNamespaces := ["kube-system"]
	ns := blockedNamespaces[_]
	elem.metadata.namespace == ns
}

chartfresh contains actionItem if {
	# Define the number of days, after which Helm releases are considered
	# to be stale.
	staleDays := 2
	not blockedNamespace(input)
	input.kind == "Secret"
	comparisonDate := time.add_date(time.now_ns(), 0, 0, 0 - staleDays)
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
		"category": "Reliability",
	}
}
