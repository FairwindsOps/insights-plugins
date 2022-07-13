package fairwinds
blockedNamespace(elem) {
    # List Kubernetes namespaces where this policy should not be applied.
    blockedNamespaces := ["kube-system"]

    ns := blockedNamespaces[_]
    elem.metadata.namespace == ns
}

# Return true if livenessProbe or ReadinessProbe has `httpGet.host` set.
probeHasExternalHost(pod) {
    probeKeys := {"livenessProbe", "readinessProbe"}

    # Get the pod-spec from each container
    container := pod.spec.containers[_]
    probeKey := probeKeys[_]
    container[probeKey].httpGet.host
}

checkCronjob[actionItem] {
    not blockedNamespace(input)
    input.kind == "CronJob"
    pod := input.spec.jobTemplate.spec.template
    probeHasExternalHost(pod)
    actionItem := {
        "title": concat(" ", [input.kind, "has one or more liveness or readiness probes using httpGet with an external host"]),
        "description": "Liveness probes that send requests to arbitrary destinations can lead to blind SSRF. [Read more](https://github.com/kubernetes/kubernetes/issues/99425)",
        "remediation": "Please do not set `httpGet.host` in a pod liveness or readiness probe",
        "category": "Security",
        "severity": 0.9,
    }
}

checkDeploymentLike[actionItem] {
    not blockedNamespace(input)
    kinds := {"Deployment", "DaemonSet", "StatefulSet", "ReplicaSet", "Job"}
    kind := kinds[_]
    input.kind == kind
    pod := input.spec.template
    probeHasExternalHost(pod)
    actionItem := {
        "title": concat(" ", [input.kind, "has one or more liveness or readiness probes using httpGet with an external host"]),
        "description": "Liveness probes that send requests to arbitrary destinations can lead to blind SSRF. [Read more](https://github.com/kubernetes/kubernetes/issues/99425)",
        "remediation": "Please do not set `httpGet.host` in a pod liveness or readiness probe",
        "category": "Security",
        "severity": 0.9,
    }
}

checkPod[actionItem] {
    not blockedNamespace(input)
    input.kind == "Pod"

    # Only alert for stand-alone pods,
    # avoiding duplicate action-items for pods which belong to a controller.
    not input.metadata.ownerReferences
    probeHasExternalHost(input)
    actionItem := {
        "title": concat(" ", [input.kind, "has one or more liveness or readiness probes using httpGet with an external host"]),
        "description": "Liveness probes that send requests to arbitrary destinations can lead to blind SSRF. [Read more](https://github.com/kubernetes/kubernetes/issues/99425)",
        "remediation": "Please do not set `httpGet.host` in a pod liveness or readiness probe",
        "category": "Security",
        "severity": 0.9,
    }
}
