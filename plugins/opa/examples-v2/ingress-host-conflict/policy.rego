package fairwinds
# Note that this policy will not function without updating the OPA plugin
# target-resource configuration to include Ingress resources`.

blockedNamespace(elem) {
	# List Kubernetes namespaces where this policy should not be applied.
	blockedNamespaces := ["kube-system"]

	ns := blockedNamespaces[_]
	elem.metadata.namespace == ns
}

contains(ingresses, elem) {
	ing := ingresses[_]
	ing.kind == elem.kind
	ing.spec.rules[_].host == elem.spec.rules[_].host
}

ingressError[actionItem] {
	not blockedNamespace(input)
	input.kind == "Ingress"

   # Get all the existing ingresses and compare hostnames
	not contains(kubernetes("ingress", "Ingress"), input)

	actionItem := {
		"title": "Ingress hostname conflict",
		"description": "The ingress object has a hostname that conflicts with an existing ingress.",
		"remediation": "Ingress hostname must be unique.",
		"category": "Reliability",
		"severity": 0.7,
	}
}
