package fairwinds
# Note that this policy will not function without updating the OPA plugin
# target-resource configuration to include Ingress resources`.

blockedNamespace(elem) {
    # List Kubernetes namespaces where this policy should not be applied.
    blockedNamespaces := ["kube-system"]

    ns := blockedNamespaces[_]
    elem.metadata.namespace == ns
}
annotationblock[actionItem] {
    # List the keys of Kubernetes annotations that will be blocked.
    blockedAnnotations := {"certmanager.k8s.io/issuer"}
    # List the Kubernetes Kinds to which this policy should apply.
    kinds := {"Ingress"}

    not blockedNamespace(input)
    kind := lower(kinds[val])
    lower(input.kind) == kind
    provided := {annotation | input.metadata.annotations[annotation]}
    missing := blockedAnnotations - provided
    found := blockedAnnotations - missing
    count(found) > 0
    actionItem := {
        "title": "Bad annotation is present",
        "description": sprintf("annotation %v is present", [found]),
        "severity": 0.1,
        "remediation": "Remove the annotation",
        "category": "Reliability"
    }
}
