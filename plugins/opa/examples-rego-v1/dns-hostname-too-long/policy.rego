package fairwinds

import rego.v1

# Note that this policy will not function without updating the OPA plugin
# target-resource configuration to include Ingress resources`.

hostNameError contains actionItem if {
	input.kind == "Ingress"

	# Create a set of all of the ingress host names in the ingress object.
	ingressHostList := {hostName | hostName := input.spec.rules[_].host}

	# Iterate over each one and check to see if their length is greater than or equal to 64 characters.
	count(ingressHostList[val]) > 63

	actionItem := {
		"title": "Ingress hostname too long",
		"description": "The ingress object has a hostname that is greater than 63 characters. RFC3280 says the maximum length of the common name should not exceed 64 characters.",
		"remediation": "Reduce the length of the hostname in the ingress object.",
		"category": "Reliability",
		"severity": 0.5,
	}
}
