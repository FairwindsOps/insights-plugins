package fairwinds
# Note that this policy will not function without updating the OPA plugin
# target-resource configuration to include Ingress resources`.
# Admission Controller must be in Active mode with OPA enabled for this policy to work.

blockedNamespace(elem) {
	# List Kubernetes namespaces where this policy should not be applied.
	blockedNamespaces := ["kube-system"]

	ns := blockedNamespaces[_]
	elem.metadata.namespace == ns
}

ingressConflict[actionItem] {
	not blockedNamespace(input)
	input.kind == "Ingress"
	# Get all the existing ingresses and compare hostnames
	allexisting := kubernetes("networking.k8s.io", "Ingress")
	existing := allexisting[_]
	oldhost := existing.spec.rules[_].host
	newhost := input.spec.rules[_].host
	newhost == oldhost

	actionItem := {
		"title": "Ingress Hostname Conflict",
		"description": sprintf("The ingress object has a hostname %q that conflicts with the existing ingress %s/%s", [newhost, existing.metadata.namespace, existing.metadata.name]),
		"remediation": "The Ingress hostname must be unique.",
		"category": "Reliability",
		"severity": 0.7
	}
}
