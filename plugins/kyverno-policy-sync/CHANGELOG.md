# Changelog

## 0.2.17
* Bump library k8s.io/apimachinery to v0.36.0
* Bump library k8s.io/client-go to v0.36.0
* Bump indirect libraries dependencies

## 0.2.16
* Bump library k8s.io/client-go to v0.35.4
* Bump indirect libraries dependencies

## 0.2.15
* Build with Go 1.26.2 (stdlib CVE-2026-32280, CVE-2026-32281, CVE-2026-32283, CVE-2026-33810) via module `go` version and `GOTOOLCHAIN=go1.26.2` in release builds.
* Pin runtime image to Alpine 3.23.4 (addresses Alpine OpenSSL CVE-2026-28390 and musl CVE-2026-40200 where applicable).

## 0.2.14
* Bump library k8s.io/apimachinery to v0.35.4

## 0.2.13
* Bump library k8s.io/apimachinery to version v0.35.3
* Bump library k8s.io/client-go to version v0.35.3
* Bump library kubectlVersion to version 1.35.3
* Bump indirect library dependencies

## 0.2.12
* Harden Alpine-based Docker images: targeted upgrades for libcrypto3, libssl3, and zlib instead of full `apk upgrade` (narrower supply-chain exposure).

## 0.2.11
* Bump indirect library dependencies

## 0.2.10
* Bump library dependencies
* Bump indirect library dependencies

## 0.2.9
* Bump kubectlVersion to v1.35.2

## 0.2.8
* Bumped to Go 1.26
* Bump library dependencies
* Bump indirect library dependencies

## 0.2.7
* Bump kubectlVersion to v1.35.1
* Bump library k8s.io/api, k8s.io/apimachinery, k8s.io/client-go to v0.35.1
* Bump library golang.org/x/crypto to v0.48.0
* Bump library golang.org/x/net to v0.50.0
* Bump library golang.org/x/term to v0.40.0
* Bump library golang.org/x/text to v0.34.0
* Bump indirect library dependencies

## 0.2.6
* Bump library github.com/klauspost/compress to v1.18.4
* Bump library golang.org/x/oauth2 to v0.35.0
* Bump library golang.org/x/sys to v0.41.0
* Bump library sigs.k8s.io/structured-merge-diff/v6 to v6.3.2
* Bump indirect library dependencies

## 0.2.5
* Bump library dependencies
* Bump indirect library dependencies

## 0.2.4
* Bump indirect library dependencies
* Fix Dockerfile kubectl version variable syntax

## 0.2.3
* Bump indirect library dependencies

## 0.2.2
* Bump library github.com/imroc/req/v3 to v3.57.0

## 0.2.1
* add `action` field to policy sync results
* refactor to use `dry-run` implementations instead of flow control
* remove the necessity of a tmp folder/file to apply policies

## 0.2.0
* Bump k8s api libraries to 0.35.0

## 0.1.20
* Bump library dependencies

## 0.1.19
* Bump library dependencies

## 0.1.18
* Refactor sync lock mechanism to use k8s lease locks

## 0.1.17
* Bumped kubectlVersion to v1.34.3

## 0.1.16
* Fixed log message

## 0.1.15
* Fixed mixing errors from other policy applies

## 0.1.14
* Improve reliability of parsePoliciesFromYAML function
* Fix typo in deriveKindFromResourceName function

## 0.1.13
* Bumped kubectl

## 0.1.12
* Bumped to go 1.25.5

## 0.1.11
* fixed some kyverno groups

## 0.1.10
* added new kyverno CRDs

## 0.1.9
* added new kyverno CRDs

## 0.1.8
* Update policy apply status

## 0.1.7
* Fix vulnerbility

## 0.1.6
* Fix policy sync bug

## 0.1.5
* Code cleanup

## 0.1.4
* Bumped go for fixing vulnerabilities

## 0.1.3
* Bumped libs version

## 0.1.2
* Fixing vulnerability

## 0.1.1
* Bug fixes

## 0.1.0
* Initial release
