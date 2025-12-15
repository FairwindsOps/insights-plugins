# Changelog

## 2.6.25
* Bump dependency libraries

## 2.6.24
* Bumped to go 1.25.5

## 2.6.23
* Bumped go for fixing vulnerabilities

## 2.6.22
* Bumped libs

## 2.6.21
* Bumped libs version

## 2.6.20
* Bumped go to 1.24.6 for fixing vulnerability

## 2.6.19
* Fixing vulnerabilities

## 2.6.18
* Update libraries

## 2.6.17
* bumped alpine to 3.22

## 2.6.16
* Added output temp file

## 2.6.15
* Bumped libs version

## 2.6.14
* Fixing vulnerabilities

## 2.6.13
* bumped alpine to 3.21

## 2.6.12
* bumped libs

## 2.6.11
* upgraded go to 1.22.7

## 2.6.10
* upgraded goreleaser to v2

## 2.6.9
* add support for go workspace

## 2.6.8
* Bump alpine to 3.20

## 2.6.7
* update dependencies

## 2.6.6
* update dependencies

## 2.6.5
* update dependencies

## 2.6.4
* update dependencies

## 2.6.3
* Bump alpine to 3.19

## 2.6.2
* update dependencies

## 2.6.1
* Update to go 1.21

## 2.6.0
* migrate from go-funk to lo

## 2.5.0
* add `PodLabels` and `PodAnnotations` to report output

## 2.4.10
* exposing workloads package version

## 2.4.9
* refactoring to make workloads a package

## 2.4.8
* workloads request set to limits if requests is not set

## 2.4.7
* update dependencies

## 2.4.6
* updated workload schema

## 2.4.5
* update alpine and x/net

## 2.4.4
* update dependencies

## 2.4.3
* update alpine and go modules

## 2.4.2
* update go modules

## 2.4.1
* update x/net and alpine

## 2.4.0
* Added ingresses support to workload report

## 2.3.4
* Update x/text to remove CVE

## 2.3.3
* update controller-utils to 0.3.0

## 2.3.2
* Fix for top-level controllers with no pod info

## 2.3.1
* Update to go 1.19

## 2.3.0
* Build docker images for linux/arm64, and update to Go 1.19.1

## 2.2.15
* upgrade plugins on build

## 2.2.14
* Update dependencies

## 2.2.13
* update to go 1.18 and update packages

## 2.2.12
* Update alpine to remove CVE

## 2.2.11
* Bump alpine to 3.16

## 2.2.10
* update versions

## 2.2.9
* update versions

## 2.2.8
* Update vulnerable packages

## 2.2.7
* Update alpine to remove CVE

## 2.2.6
* Add boolean `IsControlPlaneNode` to determine control-plane node vs worker node
## 2.2.5
* Fix go.mod `module`, and `import`s, to use plugins sub-directory.

## 2.2.4
* Update dependencies
## 2.2.3
* Update Go modules

## 2.2.2
* Bump alpine to 3.15

## 2.2.1
* Bump go modules

## 2.2.0
* Start using controller-utils to get all top workloads.

## 2.1.7
* rebuild to fix CVEs in alpine:3.14

## 2.1.6
* Update schema

## 2.1.5
* Bump dependencies and rebuild

## 2.1.4
* rebuild to fix CVEs in alpine:3.14

## 2.1.3
* rebuilt to fix CVEs in alpine 3.14

## 2.1.2
* update Go modules

## 2.1.1
* Update Go and modules

## 2.1.0
* update go dependencies

## 2.0.6
* Bump Alpine to 3.14

## 2.0.5
* Update alpine image

## 2.0.4
* Fixed a bug in the results schema

## 2.0.3

* Fixed a bug in the results schema

## 2.0.1

* Fixed a bug in the PodCount for Jobs with a nil start time or completed time.

## 2.0.0

* Set PodCount metric in the report instead of calculating on the backend.

## 1.3.0

* Updating to the new version of the kubernetes API

## 1.2.2

* Initial release
