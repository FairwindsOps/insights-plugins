# Changelog

## 0.5.14
* Fixing vulnerabilities

## 0.5.13
* Update libraries

## 0.5.12
* Bumped libs version

## 0.5.11
* Fixing vulnerabilities

## 0.5.10
* bumped libs

## 0.5.9
* upgraded go to 1.22.7

## 0.5.8
* upgraded goreleaser to v2

## 0.5.7
* add support for go workspace

## 0.5.6
* update dependencies

## 0.5.5
* update dependencies

## 0.5.4
* update dependencies

## 0.5.3
* update dependencies

## 0.5.2
* update dependencies

## 0.5.1
* Update to go 1.21

## 0.5.0
* migrate right-sizer plugin from go-funk to lo

## 0.4.9
* Update image repo for noisy-neighbor, used by E2E test workload.

## 0.4.8
* update dependencies

## 0.4.7
* update alpine and x/net

## 0.4.6
* update dependencies

## 0.4.5
* update alpine and go modules

## 0.4.4
* update go modules

## 0.4.3
* update x/net and alpine

## 0.4.2
* Update x/text to remove CVE

## 0.4.1
* Update to go 1.19

## 0.4.0
* Build docker images for linux/arm64, and update to Go 1.19.1

## 0.3.7
* Update dependencies

## 0.3.6
* update to go 1.18 and update packages

## 0.3.5
* Update alpine to remove CVE

## 0.3.4
* Update vulnerable packages

## 0.3.3
* Fix go.mod `module`, and `import`s, to use plugins sub-directory.

## 0.3.2
* Update dependencies
## 0.3.1
* Fix panic when attempting to log pod termination time from an incorrect pod-spec field (FWI-1313).
* Failure to fetch an OOM-killed pod because of a `NotFound` error is no longer logged as an error.

## 0.3.0
* Add option to filter the Kubernetes namespaces where memory limits will be updated.

## 0.2.4
* Add updating memory limits to the end-to-end test.

## 0.2.3
* Update Go modules

## 0.2.2
* Add end-to-end tests for right-sizer controller.

## 0.2.1

* Fix detecting the first OOM-kill of a pod.

## 0.2.0

* Add an option to update memory limits of the pod-controller (Deployment) in response to OOM-killed pods, using a configurable increment and maximum.
* Add a global `--namespace` flag to limit Kubernetes namespaces considered for both creating action-items and updating memory limits.
* Add an HTTP health-check endpoint, for Kubernetes readiness and liveness probes.

## 0.1.0

* Initial release

