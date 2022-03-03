package fairwinds

hostNameError[actionItem] {

  # Create a set of all of the ingress host names in the ingress object.
  ingressHostList := {hostName | hostName := input.spec.rules[_].host}

  # Iterate over each one and check to see if their length is greater than or equal to 64 characters.
  count(ingressHostList[val])  > 63

  actionItem := {
    "title": "Ingress hostname too long",
    "description": "The ingress object has a hostname that is greater than 63 characters. RFC3280 says the maximum length of the common name should not exceed 64 characters.",
    "remediation": "Reduce the length of the hostname in the ingress object.",
    "category": "Security",
    "severity": 0.99,
  }
}