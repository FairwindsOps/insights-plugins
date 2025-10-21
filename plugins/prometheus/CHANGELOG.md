# Changelog

## 1.5.16
* Bumped libs version

## 1.5.15
* Bumped go to 1.24.6 for fixing vulnerability

## 1.5.14
* Fixing vulnerabilities

## 1.5.13
* Update libraries

## 1.5.12
* bumped alpine to 3.22

## 1.5.11
* Fixed CPU bug on millicores

## 1.5.10
* Making sure only files are updaloded only when they are ready to

## 1.5.9
* Making kube state metrics non zero validation off by default

## 1.5.8
* Bumped libs version

## 1.5.7
* Fixing vulnerabilities

## 1.5.6
* bumped alpine to 3.21

## 1.5.5
* bumped libs

## 1.5.4
* upgraded prometheus lib

## 1.5.3
* upgraded go to 1.22.7

## 1.5.2
* upgraded goreleaser to v2

## 1.5.1
* add support for go workspace

## 1.5.0
* added all-zero validation for ksm and cAdvisor

## 1.4.8
* Bump alpine to 3.20

## 1.4.7
* round prometheus metrics to 1 milli CPU

## 1.4.6
* added overhead costs to prometheus plugin

## 1.4.5
* google managed prometheus cluster filter

## 1.4.4
* Added google access token to prometheus plugin

## 1.4.3
* update dependencies

## 1.4.2
* update dependencies

## 1.4.1
* update dependencies

## 1.4.0
* Added CPU and Memory idle data

## 1.3.8
* update dependencies

## 1.3.7
* Bump alpine to 3.19

## 1.3.6
* update dependencies

## 1.3.5
* Update to go 1.21

## 1.3.4
* Prometheus bug fix

## 1.3.3
* storage metric bug fix

## 1.3.2
* fixing network transmit and receive

## 1.3.1
* update dependencies

## 1.3.0
* Add storage capacity to the metrics submitted to Insights.

## 1.2.0
* Add network transmit bytes, and network received bytes, to the metrics submitted to Insights.
* Add ability to output debug logs using the `LOGRUS_LEVEL` environment variable.

## 1.1.9
* update alpine and x/net

## 1.1.8
* update dependencies

## 1.1.7
* update alpine and go modules

## 1.1.6
* update go modules

## 1.1.5
* update x/net and alpine

## 1.1.4
* Update x/text to remove CVE

## 1.1.3
* update controller-utils to 0.3.0

## 1.1.2
* Update to go 1.19

## 1.1.1
* improve pod owner matching

## 1.1.0
* Build docker images for linux/arm64, and update to Go 1.19.1

## 1.0.7
* upgrade plugins on build

## 1.0.6
* Update dependencies

## 1.0.5
* update to go 1.18 and update packages

## 1.0.4
* Update alpine to remove CVE

## 1.0.3
* Bump alpine to 3.16

## 1.0.2
* update versions

## 1.0.1
* Update vulnerable packages

## 1.0.0
* rename output file from `resource-metrics.json` to `prometheus-metrics.json`

## 0.4.15
* Update alpine to remove CVE

## 0.4.14
* Fix go.mod `module`, and `import`s, to use plugins sub-directory.

## 0.4.13
* Update dependencies

## 0.4.12
* Update Go modules

## 0.4.11
* Bump alpine to 3.15
## 0.4.10
* Bump go modules

## 0.4.9
* rebuild to fix CVEs in alpine:3.14

## 0.4.8
* Stop skipping data for non-/kubepod prefixes

## 0.4.7
* Bump dependencies and rebuild

## 0.4.6
* Added Request and Limit values
* Modify resource id format to cater for new prometheus and kubernetes versions(v1.21.1)

## 0.4.5
* Add some logs

## 0.4.4
* rebuild to fix CVEs in alpine:3.14

## 0.4.3
* rebuilt to fix CVEs in alpine 3.14

## 0.4.2
* update Go modules

## 0.4.1
* Update Go and modules

## 0.4.0
* update go dependencies

## 0.3.3
* Bump Alpine to 3.14

## 0.3.2

* Only retrieve 1.5x data instead of 2x data.

## 0.3.1

* Update alpine image

## 0.3.0

* Change the model of the output into an object

## 0.2.0

* Changed the path of the output file

## 0.1.0

* Initial release
