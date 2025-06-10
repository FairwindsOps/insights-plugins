# Changelog

## 0.3.9
* revert: added validation policy reports and policies to report

## 0.3.8
* added validation policy reports and policies to report

## 0.3.7
* bumped alpine to 3.22

## 0.3.6
* Added output temp file

## 0.3.5
* Bumped libs version

## 0.3.4
* Fixing vulnerabilities

## 0.3.3
* bumped alpine to 3.21

## 0.3.2
* upgraded go to 1.22.7

## 0.3.1
* upgraded goreleaser to v2

## 0.3.0
* Converted kyverno plugin to golang

## 0.2.1
* Bump alpine to 3.20

## 0.2.0
* Accommodate the change in Kyverno report aggregation in v1.11+ (https://github.com/kyverno/kyverno/pull/8426)

## 0.1.5
* Bump kubectl to 1.29.0

## 0.1.4
* Bump alpine to 3.19

## 0.1.3
* Modify script to use `jq` `--slurpfile` flag instead of a HERESTRING, to avoid "argument list too long" error
## 0.1.2
* Fix log message when checking for clusterreports
* Include policy/clusterpolicy title and description in report
* bash refactoring

## 0.1.1
* add missing CRD check to Kyverno script, fix entrypoint for Dockerfile

## 0.1.0
* Initial release
