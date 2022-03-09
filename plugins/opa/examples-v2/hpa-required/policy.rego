package fairwinds

blockedNamespace(elem) {
    # List Kubernetes namespaces where this policy should not be applied.
    blockedNamespaces := ["kube-system"]

    ns := blockedNamespaces[_]
    elem.metadata.namespace == ns
}

contains(hpas, elem) {
    hpa := hpas[_]
    hpa.spec.scaleTargetRef.kind == elem.kind
    hpa.spec.scaleTargetRef.name == elem.metadata.name
    hpa.metadata.namespace == elem.metadata.namespace
    hpa.spec.scaleTargetRef.apiVersion == elem.apiVersion
}

hparequired[actionItem] {
    not blockedNamespace(input)
    input.kind == "Deployment"
    not contains(kubernetes("autoscaling", "HorizontalPodAutoscaler"), input)
    actionItem := {
        "title": "HPA is required",
        "description": "No horizontal pod autoscaler found",
        "severity": 0.1,
        "remediation": "Create an HPA",
        "category": "Reliability"
    }
}
