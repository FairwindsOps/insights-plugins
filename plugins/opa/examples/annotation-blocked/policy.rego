package fairwinds
blockedNamespace(elem) {
    ns := elem.parameters.blocklist[_]
    elem.metadata.namespace == ns
}
annotationblock[actionItem] {
    not blockedNamespace(input)
    provided := {annotation | input.metadata.annotations[annotation]}
    required := {annotation | annotation := input.parameters.annotations[_]}
    missing := required - provided
    found := required - missing
    count(found) > 0
    description := sprintf("annotation %v is present", [found])
    actionItem := {
        "title": "Bad annotation is present",
        "description": sprintf("annotation %v is present", [found]),
        "severity": 0.1,
        "remediation": "Remove the annotation",
        "category": "Reliability"
    }
}
