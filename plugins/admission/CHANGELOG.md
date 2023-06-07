# Changelog

## 1.10.2
* Display warnings for items that would have blocked when admission is in "passive" mode
* Update alpine base image from `alpine:3.17` to `alpine:3.18`

## 1.10.1
* Fix webhook server `cert-dir` and `port` after `sigs.k8s.io/controller-runtime` upgrade

## 1.10.0
* Update dependencies (polaris 8.0.0)

## 1.9.9
* Update dependencies

## 1.9.8
* Show message from admission request

## 1.9.7
* update dependencies

## 1.9.6
* update alpine and x/net

## 1.9.5
* update dependencies

## 1.9.4
* update alpine and go modules

## 1.9.3
* Revert v1.9.2

## 1.9.2
* Update pluto from 5.9 to 5.12
* Update polaris from 20230104151009-8af436367263 to 20230105172421-bf065f9b5455

## 1.9.1
* update go modules

## 1.9.0
* Fix Polaris mutations with the Insights admission controller, including updating to current Polaris code and its default configuration file (which contains the `mutations` block)
* Update Polaris to `20221114220502-467d06f4dbca` from `20220512134546-92f0b6e551df`
* A bump for k8s.io/apimachinery and sigs.k8s.io/controller-runtime while troubleshooting

## 1.8.3
* update x/net and alpine

## 1.8.2
* Pass the admission request object to the OPA runtime engine

## 1.8.1
* Update x/text to remove CVE

## 1.8.0
* Added support to ignore some services account

## 1.7.1
* Update to go 1.19

## 1.7.0
* Build docker images for linux/arm64, and update to Go 1.19.1

## 1.6.0
* adds `namespaceMetadata` field to `metadata` report 

## 1.5.6
* upgrade plugins on build

## 1.5.5
* Update dependencies

## 1.5.4
* update to go 1.18 and update packages

## 1.5.3
* Update alpine to remove CVE

## 1.5.2
* Fix admission-controller bug where Pluto deprecation/removal were not being populated.

## 1.5.1
* Improve Docker image rebuilding by using mount-cache.

## 1.5.0
* Update admission controller to support Pluto

## 1.4.1
* update versions

## 1.4.0
* Added polaris mutation option

## 1.3.7
* Bump alpine to 3.16

## 1.3.6
* update versions

## 1.3.5
* update versions

## 1.3.4
* Trivy bug fix

## 1.3.3
* Update vulnerable packages

## 1.3.2
* Update vulnerable packages

## 1.3.1
* Update alpine to remove CVE

## 1.3.0
* Add a `version` package to reflect the plugin version in reports, and send the current plugin version to the API.

## 1.2.2
* No longer deny admission requests if errors are returned by plugins and the Kubernetes webhook failure policy is set to `Ignore`. The failure policy is passed via the `WEBHOOK_FAILURE_POLICY` environment variable.

## 1.2.1
* Fix go.mod `module`, and `import`s, to use plugins sub-directory.

## 1.2.0

* The cluster name is now correctly available via the `insightsinfo("cluster")` rego function.
* Processing of checks will now continue when there has been a failure, to collect and output all failure conditions. Multiple errors may be reflected in both admission webhook output and in plugin log output.
* Process v2 CustomChecks, which lack the Insights Instance yaml accompanying the rego policy.

## 1.1.0
* Add an `insightsinfo` function to make Insights information available in rego.

## 1.0.0
* Bump plugin version

## 0.5.2
* Update dependencies

## 0.5.1
* Update OPA plugin to support removal of CRD

## 0.5.0
* Update Polaris to version 5.0.0

## 0.4.7
* Make webhook port configurable via env variable `WEBHOOK_PORT`

## 0.4.6
* Add support for log level configuration
* Add more information when insights request fails
* Remove resetting object and oldObject structs

## 0.4.5
* Update Go modules

## 0.4.4
* Add some logging for OPA

## 0.4.3
* Bump alpine to 3.15

## 0.4.2
* Bump go modules

## 0.4.1
* rebuild to fix CVEs in alpine:3.14

## 0.4.0
* Update Polaris to the latest version
## 0.3.6
* Bump dependencies and rebuild

## 0.3.5
* rebuild to fix CVEs in alpine:3.14

## 0.3.4
* rebuilt to fix CVEs in alpine 3.14
## 0.3.3
* Add some logging

## 0.3.2
* update Go modules

## 0.3.1
* Update Go and modules

## 0.3.0
* update go dependencies

## 0.2.3
* Bump Alpine to 3.14

## 0.2.2
* Update alpine image

## 0.2.1

* Added `HelmName` to the model

## 0.2.0

* Added metadata report

## 0.1.2

* Logging adjustments

## 0.1.1

* Initial release

