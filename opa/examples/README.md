# OPA Examples

The files in this directory are example Custom Checks to be used with Fairwinds Insights.

You can install one of these into your Fairwinds Insights organization with a command like this.

`curl -H "Content-type: application/x-yaml" -H "Authorization: Bearer $admintoken" https://insights.fairwinds.com/v0/organizations/$organization/opa/customChecks/$checkName -X PUT --data-binary @label-required.yaml`

In order to use one of these checks you also need a target for one of them. With a file like this
```
targets:
- apiGroups: [""]
  kinds: ["Namespace"]
parameters:
  labels: ["hr"]
output:
  title: "Namespace Label is missing"
  severity: 0.2
  remediation: "Add the label to your namespace"
```

You could send that to Insights with this curl
`curl -H "Content-type: application/x-yaml" -H "Authorization: Bearer $token" https://insights.fairwinds.com/v0/organizations/$organization/opa/customChecks/$checkName/$instanceName -X PUT --data-binary @hr.yaml`