package fairwinds

blockedNamespace(elem) {
    # List Kubernetes namespaces where this policy should not be applied.
    blockedNamespaces := ["kube-system"]

    ns := blockedNamespaces[_]
    elem.metadata.namespace == ns
}

# If this block evaluates to true an action item is created
rollingUpdate[actionItem] {

    # Checks to see if this resource should be skipped or not based on blocked namespaces.
    not blockedNamespace(input)

    input.kind == "Deployment"

    # These lines check to see if the maxUnavailable field is a percentage, and if so to strip it and converts it to a number for evaluation.
    maxUnavailable := input.spec.strategy.rollingUpdate.maxUnavailable
    contains(maxUnavailable, "%")
    maxUnavailble = replace(maxUnavailable, "%", "")
    newMaxUnavailable := to_number(maxUnavailble)
 
    # If newMaxUnavailable is greater than 25 then create an action item.
    newMaxUnavailable > 25

    actionItem := {
        "title": "Rolling Update policy is too aggressive",
        "description": sprintf("The current deployment strategy for this resource has a value of %v, which exceeds the maximum value of 25%% or less as required by this policy.", [maxUnavailable]),
        "severity": 0.6,
        "remediation": "Change maxUnavailable to be less than or equal to 25%",
        "category": "Reliability"
    }
}
