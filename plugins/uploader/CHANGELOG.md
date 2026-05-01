# Changelog

## 0.6.8
* Improving renovate

## 0.6.7
* Build with Go 1.26.2 (stdlib CVE-2026-32280, CVE-2026-32281, CVE-2026-32283, CVE-2026-33810) via module `go` version and `GOTOOLCHAIN=go1.26.2` in release builds.
* Pin runtime image to Alpine 3.23.4 (addresses Alpine OpenSSL CVE-2026-28390 and musl CVE-2026-40200 where applicable).

## 0.6.6
* Bump library kubectlVersion to version 1.35.3

## 0.6.5
* Harden Alpine-based Docker images: targeted upgrades for libcrypto3, libssl3, and zlib instead of full `apk upgrade` (narrower supply-chain exposure).

## 0.6.4
* Fixing vulnerabilities

## 0.6.3
* Bump kubectlVersion to v1.35.2

## 0.6.2
* Bump kubectlVersion to v1.35.1

## 0.6.1
* Fix the URL structure to download kubectl

## 0.6.0
* Bump k8s api libraries to 0.35.0

## 0.5.14
* Bump library dependencies

## 0.5.13
* Updated kubectlVersion to 1.34.2

## 0.5.12
* Alpine 3.23

## 0.5.11
* Bumped go to 1.24.6 for fixing vulnerability

## 0.5.10
* Fixing vulnerabilities

## 0.5.9
* bumped alpine to 3.22

## 0.5.8
* bumped alpine to 3.21

## 0.5.7
* fixing vulnerabilities

## 0.5.6
* upgraded goreleaser to v2

## 0.5.5
* Bump alpine to 3.20

## 0.5.4
* Changed curl option to --data-binary to support big files

## 0.5.3
* Bump kubectl to 1.29.0

## 0.5.2
* Bump alpine to 3.19
## 0.5.1
* update dependencies

## 0.5.0
* add `CURL_EXTRA_ARGS` option

## 0.4.3
* update alpine and x/net

## 0.4.2
* update dependencies

## 0.4.1
* update x/net and alpine

## 0.4.0
* Build docker images for linux/arm64, and update to Go 1.19.1

## 0.3.16
* upgrade plugins on build

## 0.3.15
* Update dependencies

## 0.3.14
* Update alpine to remove CVE

## 0.3.13
* Bump alpine to 3.16

## 0.3.12
* update versions

## 0.3.11
* Update vulnerable packages

## 0.3.10
* Update alpine to remove CVE
## 0.3.9
* Update dependencies

## 0.3.8
* remove debug log

## 0.3.7
* Bump alpine to 3.15

## 0.3.6
* rebuild to fix CVEs in alpine:3.14

## 0.3.5
* rebuild to fix CVEs in alpine:3.14

## 0.3.4
* rebuilt to fix CVEs in alpine 3.14

## 0.3.3
* Bump Alpine to 3.14

## 0.3.2

* Fix typo in curl for download script

## 0.3.1

* Fix a bug in the download script to not fail on 404

## 0.3.0

* Add script to download a config file

## 0.2.5

* Update alpine image

## 0.2.4

* Fixed a bug in the failure uploads

## 0.2.3

* Fixed a bug in math

## 0.2.2

* Decrease verbosity of logging unless DEBUG environment variable is set.
* Fix bug
* Increase verbosity of timeout message

## 0.2.0

* Send error logs when a container fails.

## 0.1.3

* Initial release
