# Changelog

## 0.2.16
* Bump library k8s.io/api to v0.36.0
* Bump library k8s.io/apimachinery to v0.36.0
* Bump library k8s.io/client-go to v0.36.0
* Bump indirect libraries dependencies

## 0.2.15
* Bump library k8s.io/api to v0.35.4
* Bump library k8s.io/client-go to v0.35.4

## 0.2.14
* Build with Go 1.26.2 (stdlib CVE-2026-32280, CVE-2026-32281, CVE-2026-32283, CVE-2026-33810) via module `go` version and `GOTOOLCHAIN=go1.26.2` in release builds.
* Pin runtime image to Alpine 3.23.4 (addresses Alpine OpenSSL CVE-2026-28390 and musl CVE-2026-40200 where applicable).

## 0.2.13
* Bump library k8s.io/apimachinery to v0.35.4

## 0.2.12
* Bump library k8s.io/api to version v0.35.3
* Bump library k8s.io/apimachinery to version v0.35.3
* Bump library k8s.io/client-go to version v0.35.3
* Bump indirect library dependencies

## 0.2.11
* Harden Alpine-based Docker images: targeted upgrades for libcrypto3, libssl3, and zlib instead of full `apk upgrade` (narrower supply-chain exposure).

## 0.2.10
* Bump indirect library dependencies

## 0.2.9
* Bump library dependencies
* Bump indirect library dependencies

## 0.2.8
* Bump library dependencies
* Bump indirect library dependencies

## 0.2.7
* Bumped to Go 1.26
* Bump library dependencies
* Bump indirect library dependencies

## 0.2.6
* Bump indirect library dependencies

## 0.2.5
* Bump library github.com/klauspost/compress to v1.18.4
* Bump library golang.org/x/oauth2 to v0.35.0
* Bump library golang.org/x/sys to v0.41.0
* Bump library sigs.k8s.io/structured-merge-diff/v6 to v6.3.2
* Bump indirect library dependencies

## 0.2.4
* Bump library dependencies
* Bump indirect library dependencies

## 0.2.3
* Bump indirect library dependencies

## 0.2.2
* Bump indirect library dependencies

## 0.2.1
* Bump library github.com/imroc/req/v3 to v3.57.0

## 0.2.0
* Bump k8s api libraries to 0.35.0

## 0.1.10
* Bump library dependencies

## 0.1.9
* Bump library dependencies

## 0.1.8
* Add support for `backoffLimit` and set no-retries for `kyverno-policy-sync`

## 0.1.7
* Bumped to go 1.25.5

## 0.1.6
* Bumped go for fixing vulnerabilities

## 0.1.5
* Bumped libs version

## 0.1.4
* Added kyverno policies

## 0.1.3
* Fixing vulnerability

## 0.1.2
* Bumped go to 1.24.6 for fixing vulnerability

## 0.1.1
* Expose `pollInterval` configuration

## 0.1.0
* Initial release
