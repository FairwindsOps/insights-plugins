# Changelog

## 0.2.5

* Add option to filter the Kubernetes namespaces where memory limits will be updated.

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

