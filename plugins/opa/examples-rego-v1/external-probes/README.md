# External Liveness or Readiness Probes

This policy matches pod specifications that specify an external host in a livenessProbe or readinessProbe, which may be undesirable RE: [Pod probes lead to blind SSRF from the node  #99425](https://github.com/kubernetes/kubernetes/issues/99425).
