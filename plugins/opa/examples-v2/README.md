# OPA Examples

The files in this directory are example Custom Checks to be used with Fairwinds Insights.

Most of these policies have configurable variables at the top of each rule, to help customize policy behavior such as Kubernetes Kinds to which the policy should apply, or Kubernetes Namespaces which should be excempt from the policy.

These policies can be installed into Fairwinds Insights using one of the following options:

* From the Insights user interface, see the `Create From Template` button under the Policy section.
* Use the [Insights command-line](https://github.com/FairWindsOps/insights-cli) to upload policies using the `insights-cli policy sync` command.
* Add a policy directly using the Insights API - for example: `curl --data-binary @filename.rego -X PUT -H "Content-type: text/plain" -H "Authorization: Bearer $FAIRWINDS_TOKEN" "https://insights.fairwinds.com/v0/organizations/$organization/opa/customChecks/$checkName?version=2.0"`
  * Replace `$FAIRWINDS_TOKEN` with an organization API token.
  * Replace `$organization` with an existing organization name in Insights.
  * Replace `$checkName` with what you would like to name the new custom check in Insights. This name will show up in the action items created by the check.

## Testing Policies

Each policy contains a `test-manifest` sub-directory with Kubernetes manifests to help test and demonstrate what the policy should match.
