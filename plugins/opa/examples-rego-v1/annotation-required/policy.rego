package fairwinds

import rego.v1

blockedNamespace(elem) if {
	# List Kubernetes namespaces where this policy should not be applied.
	blockedNamespaces := ["kube-system"]

	ns := blockedNamespaces[_]
	elem.metadata.namespace == ns
}

annotationrequired contains actionItem if {
	# List the keys of Kubernetes annotations that will be required.
	requiredAnnotations := {"meta.helm.sh/release-name"}

	# List the Kubernetes Kinds to which this policy should apply.
	kinds := {"Deployment"}

	not blockedNamespace(input)
	kind := lower(kinds[val])
	lower(input.kind) == kind
	provided := {annotation | input.metadata.annotations[annotation]}
	missing := requiredAnnotations - provided
	count(missing) > 0
	actionItem := {
		"title": "Annotation is missing",
		"description": sprintf("Annotation %v is missing", [missing]),
		"severity": 0.1,
		"remediation": "Add the annotation",
		"category": "Reliability",
	}
}
