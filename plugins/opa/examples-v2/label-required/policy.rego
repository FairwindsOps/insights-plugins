package fairwinds

blockedNamespace(elem) {
    # List Kubernetes namespaces where this policy should not be applied.
    blockedNamespaces := ["kube-system"]

    ns := blockedNamespaces[_]
    elem.metadata.namespace == ns
}

labelrequired[actionItem] {
    # List the keys of Kubernetes labels that will be required.
    requiredLabels := {"app"}
    # List the Kubernetes Kinds to which this policy should apply.
    kinds := {"Deployment", "DaemonSet", "StatefulSet", "CronJob", "Job"}

    not blockedNamespace(input)
    kind := lower(kinds[val])
    lower(input.kind) == kind
    provided := {label | input.metadata.labels[label]}
    missing := requiredLabels - provided
    count(missing) > 0
    description := sprintf("Label %v is missing", [missing])
    severity := 0.1 * count(missing)
    actionItem := {
        "title": "Label is missing",
        "description": description,
        "severity": severity,
        "remediation": "Add the label",
        "category": "Reliability"
    }
}
