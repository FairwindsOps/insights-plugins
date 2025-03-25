package fairwinds

import rego.v1

blockedNamespace(elem) if {
	# List Kubernetes namespaces where this policy should not be applied.
	blockedNamespaces := ["kube-system"]

	ns := blockedNamespaces[_]
	elem.metadata.namespace == ns
}

containsHPA(hpas, elem) if {
	hpa := hpas[_]
	hpa.spec.scaleTargetRef.kind == elem.kind
	hpa.spec.scaleTargetRef.name == elem.metadata.name
	hpa.metadata.namespace == elem.metadata.namespace
	hpa.spec.scaleTargetRef.apiVersion == elem.apiVersion
}

hparequired contains actionItem if {
	not blockedNamespace(input)
	input.kind == "Deployment"
	not containsHPA(kubernetes("autoscaling", "HorizontalPodAutoscaler"), input)
	actionItem := {
		"title": "HPA is required",
		"description": "No horizontal pod autoscaler found",
		"severity": 0.1,
		"remediation": "Create an HPA",
		"category": "Reliability",
	}
}
