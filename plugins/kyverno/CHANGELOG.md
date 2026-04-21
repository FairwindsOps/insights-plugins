# Changelog

## 0.5.15
* Build with Go 1.26.2 (stdlib CVE-2026-32280, CVE-2026-32281, CVE-2026-32283, CVE-2026-33810) via module `go` version and `GOTOOLCHAIN=go1.26.2` in release builds.
* Pin runtime image to Alpine 3.23.4 (addresses Alpine OpenSSL CVE-2026-28390 and musl CVE-2026-40200 where applicable).

## 0.5.14
* Bump library k8s.io/apimachinery to v0.35.4

## 0.5.13
* Bump library k8s.io/apimachinery to version v0.35.3
* Bump library k8s.io/client-go to version v0.35.3
* Bump indirect library dependencies

## 0.5.12
* Harden Alpine-based Docker images: targeted upgrades for libcrypto3, libssl3, and zlib instead of full `apk upgrade` (narrower supply-chain exposure).

## 0.5.11
* Bump indirect library dependencies

## 0.5.10
* Bump library dependencies
* Bump indirect library dependencies

## 0.5.8
* Bumped to Go 1.26
* Bump library dependencies
* Bump indirect library dependencies

## 0.5.7
* Bump library k8s.io/api, k8s.io/apimachinery, k8s.io/client-go to v0.35.1
* Bump library golang.org/x/net to v0.50.0
* Bump library golang.org/x/term to v0.40.0
* Bump library golang.org/x/text to v0.34.0
* Bump indirect library dependencies

## 0.5.6
* Bump library golang.org/x/oauth2 to v0.35.0
* Bump library golang.org/x/sys to v0.41.0
* Bump library sigs.k8s.io/structured-merge-diff/v6 to v6.3.2
* Bump indirect library dependencies

## 0.5.5
* Bump library dependencies
* Bump indirect library dependencies

## 0.5.4
* Bump indirect library dependencies

## 0.5.3
* Bumped all libs

## 0.5.2
* Bump indirect library dependencies

## 0.5.1
* Bump library dependencies

## 0.5.0
* Bump k8s api libraries to 0.35.0

## 0.4.9
* Bump library dependencies

## 0.4.8
* Bump library dependencies

## 0.4.7
* Bumped to go 1.25.5

## 0.4.6
* Fix kyverno report descriptions

## 0.4.5
* Code cleanup

## 0.4.4
* Bumped go for fixing vulnerabilities

## 0.4.3
* Bumped libs version

## 0.4.2
* Bumped go to 1.24.6 for fixing vulnerability

## 0.4.1
* fixing vulnerabilities

## 0.4.0
* added validation policy reports and policies to report

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
