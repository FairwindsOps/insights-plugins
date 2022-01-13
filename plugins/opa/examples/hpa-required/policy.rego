package fairwinds

contains(hpas, elem) {
    hpa := hpas[_]
    hpa.spec.scaleTargetRef.kind == elem.kind
    hpa.spec.scaleTargetRef.name == elem.metadata.name
    hpa.metadata.namespace == elem.metadata.namespace
    hpa.spec.scaleTargetRef.apiVersion == elem.apiVersion
}

blockedNamespace(elem) {
    ns := elem.parameters.blocklist[_]
    elem.metadata.namespace == ns
}

hparequired[actionItem] {
    not blockedNamespace(input)
    not contains(kubernetes("autoscaling", "HorizontalPodAutoscaler"), input)
    actionItem := {
        "title": "HPA is required",
        "description": "No horizontal pod autoscaler found",
        "severity": 0.1,
        "remediation": "Create an HPA",
        "category": "Reliability"
    }
}
