# Changelog

## 2.2.1
* Add docker-socket check opa policy template.

## 2.2.0
* Add support for `insightsinfo("admissionRequest")` that exposes the admission request

## 2.1.2
* Update x/text to remove CVE

## 2.1.1
* Update to go 1.19

## 2.1.0
* Build docker images for linux/arm64, and update to Go 1.19.1

## 2.0.18
* upgrade plugins on build

## 2.0.17
* Update dependencies

## 2.0.16
* support HPA v2beta1 for local files

## 2.0.15
* update to go 1.18 and update packages

## 2.0.14
* Update alpine to remove CVE

## 2.0.13
* Validate modified OPA policies in CI.

## 2.0.12
* update versions

## 2.0.11
* Bump alpine to 3.16

## 2.0.10
* update versions

## 2.0.9
* Update vulnerable packages

## 2.0.8
* Update vulnerable packages

## 2.0.7
* Update alpine to remove CVE

## 2.0.6
* Add a `version` package to reflect the plugin version in reports.

## 2.0.4
* Add V2 varients of example CustomCheck policies.

## 2.0.3
* Fix checks always logging an error indicating "0 errors occurred."

## 2.0.2
* Adding an OPA policy to catch DNS hostnames longer than 63 characters.

## 2.0.1
* Fix go.mod `module`, and `import`s, to use plugins sub-directory.

## 2.0.0

* Processing of checks is no longer interrupted by a failure to list objects for one of the checks Kube targets. The remaining targets will be checked, and errors reflected in the plugin log.
  * Only 3 errors are logged per check, to avoid overwhelming logs. Enable OPA plugin debug logging to log errors involving all objects.
* Process v2 CustomChecks, which use a list of Kubernetes APIGroup/Kind passed to the OPA plugin, instead of Insights Instance yaml accompanying the rego policy.
* Add command-line options to specify Kubernetes resource targets for V2 custom checks, and to enable debug logging.
* Use the CustomCheck name as the `EventType`, instead of the Check Instance name.

## 1.0.2
* Errors from the `kubernetes` function now cause rego to fail, and log warnings.
* Errors processing OPA policies are no longer logged multiple times, and are bundled and returned at the end of the plugin run.
* Add a `insightsinfo` function that makes Insights information available in policies.

## 1.0.1
* Update dependencies
## 1.0.0
* Remove Opa CRD

## 0.3.14
* Update policy examples to have consistent indentation

## 0.3.13
* Add additional policy examples related to CVE-2021-43816

## 0.3.12
* Update Go modules

## 0.3.11
* Add some logging

## 0.3.10
* Add additional example policies for roll-out strategies

## 0.3.9
* Bump alpine to 3.15

## 0.3.8
* Bump go modules

## 0.3.7
* rebuild to fix CVEs in alpine:3.14

## 0.3.6
* Update `label-required` policy example

## 0.3.5
* Bump dependencies and rebuild

## 0.3.4
* rebuild to fix CVEs in alpine:3.14

## 0.3.3
* rebuilt to fix CVEs in alpine 3.14

## 0.3.2
* update Go modules

## 0.3.1
* Update Go and modules

## 0.3.0
* update go dependencies

## 0.2.23
* Bump Alpine to 3.14

## 0.2.22

* Add `cert-manager` policy example

## 0.2.21

* Ignore custom checks created by another Insights Agent.

## 0.2.20

* Add `external-probes` policy

## 0.2.19

* Update alpine image

## 0.2.18

* add some more examples

## 0.2.17

* Reformat policies into .rego files

## 0.2.16

* Check for already exists error

## 0.2.15

* Fixed typo on remediation

## 0.2.14

* Added additional example policies for vulnerabilities.
* Fixed typo on remediation

## 0.2.13

* Added additional example policies

## 0.2.12

* Fix a bug when using files as source for Kube client

## 0.2.11
* Code refactor

## 0.2.10
* Added tests for examples

## 0.2.8
* Refactoring code

## 0.2.6
* Update examples

## 0.2.3
* Refresh checks before retrieving them
* Added some logging

## 0.2.2
* Fixed bug for null parameters

## 0.2.1
* Added examples

## 0.2.0
* Dynamically load policies

## 0.1.0
* Initial release
