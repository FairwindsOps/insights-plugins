# Changelog

## 0.34.19
* Bump library k8s.io/api to v0.35.4
* Bump library k8s.io/client-go to v0.35.4

## 0.34.18
* Build with Go 1.26.2 (stdlib CVE-2026-32280, CVE-2026-32281, CVE-2026-32283, CVE-2026-33810) via module `go` version and `GOTOOLCHAIN=go1.26.2` in release builds.
* Pin runtime image to Alpine 3.23.4 (addresses Alpine OpenSSL CVE-2026-28390 and musl CVE-2026-40200 where applicable).
* Bump bundled Trivy CLI to v0.70.0 (addresses upstream CVEs in bundled dependencies including stdlib, gRPC, Docker, and BuildKit).
* Harden the Trivy CLI downloader stage: `apk upgrade` plus explicit `musl` / `musl-utils` upgrades alongside OpenSSL/zlib.

## 0.34.17
* Bump library k8s.io/apimachinery to v0.35.4

## 0.34.16
* Bump library github.com/google/go-containerregistry to version v0.21.5
* Bump indirect library dependencies

## 0.34.15
* Bump library github.com/google/go-containerregistry to version v0.21.4
* Bump indirect library dependencies

## 0.34.14
* Bump library github.com/google/go-containerregistry to version v0.21.3
* Bump library k8s.io/api to version v0.35.3
* Bump library k8s.io/apimachinery to version v0.35.3
* Bump library k8s.io/client-go to version v0.35.3
* Bump indirect library dependencies

## 0.34.13
* Harden Alpine-based Docker images: targeted upgrades for libcrypto3, libssl3, and zlib instead of full `apk upgrade` (narrower supply-chain exposure).

## 0.34.12
* Bump library CLOUD_SDK_VERSION to version 560.0.0
* Bump indirect library dependencies

## 0.34.11
* Bump trivy to 0.69.3, CLOUD_SDK_VERSION to 559.0.0
* Bump library dependencies
* Bump indirect library dependencies

## 0.34.10
* Bump library dependencies
* Bump indirect library dependencies

## 0.34.9
* Remove kubectl from image to fix CVE-2025-68121 (Go crypto/tls in kubectl binary). Plugin uses in-cluster Kubernetes client, not kubectl.
* Bumped CLOUD_SDK_VERSION to 558.0.0

## 0.34.8
* Bumped to Go 1.26
* Bump library dependencies
* Bump indirect library dependencies

## 0.34.7
* Bump kubectlVersion to v1.35.1
* Bump CLOUD_SDK_VERSION to 556.0.0
* Bump library k8s.io/api, k8s.io/apimachinery, k8s.io/client-go to v0.35.1
* Bump library golang.org/x/net to v0.50.0
* Bump library golang.org/x/term to v0.40.0
* Bump library golang.org/x/text to v0.34.0
* Bump indirect library dependencies

## 0.34.6
* Bump trivy to 0.69.1
* Bump library github.com/docker/cli to v29.2.1
* Bump library github.com/klauspost/compress to v1.18.4
* Bump library golang.org/x/oauth2 to v0.35.0
* Bump library golang.org/x/sys to v0.41.0
* Bump library sigs.k8s.io/structured-merge-diff/v6 to v6.3.2
* Bump indirect library dependencies

## 0.34.5
* Bump trivy to 0.69.0
* Bump CLOUD_SDK to 554.0.0
* Bump library github.com/docker/cli to v29.2.0
* Bump indirect library dependencies

## 0.34.4
* Bump indirect library dependencies

## 0.34.3
* Bumped all libs

## 0.34.2
* Bump indirect library dependencies

## 0.34.1
* Bump library dependencies

## 0.34.0
* Bump k8s api libraries to 0.35.0

## 0.33.12
* Bump library dependencies

## 0.33.11
* Bump library dependencies

## 0.33.10
* Updated kubectlVersion to 1.34.2

## 0.33.9
* Bumped to go 1.25.5

## 0.33.8
* Bumped trivy for fixing vulnerabilities

## 0.33.7
* Bumped go for fixing vulnerabilities

## 0.33.6
* Bumped libs

## 0.33.5
* Bumped libs version

## 0.33.4
* Bump trivy to 0.67.0

## 0.33.3
* Bump trivy to 0.66.0

## 0.33.2
* Bumped go to 1.24.6 for fixing vulnerability

## 0.33.1
* Bump trivy to 0.65.0

## 0.33.0
* Added support to trivy server

## 0.32.1
* Bump trivy to 0.64.1

## 0.32.0
* Add support for `IMAGES_TO_SCAN` env var to specify a comma-separated list of images to scan

## 0.31.16
* Fixing vulnerabilities

## 0.31.15
* fixing vulnerabilities

## 0.31.14
* updated image to python-alpine

## 0.31.13
* Update libraries

## 0.31.12
* bumped alpine to 3.22

## 0.31.11
* update trivy to 0.62.1

## 0.31.10
* update trivy to 0.61.0

## 0.31.9
* Fixed trivy vulnerability

## 0.31.8
* Fixed trivy vulnerability

## 0.31.7
* Bumped libs version

## 0.31.6
* Fixing vulnerabilities

## 0.31.5
* bumped alpine to 3.21

## 0.31.4
* bumped libs

## 0.31.3
* bumped trivy to 0.57.1

## 0.31.2
* fix trivy db / java-db cache

## 0.31.1
* bumped trivy to 0.57.0

## 0.31.0
* Add new env. variable support `SERVICE_ACCOUNT_ANNOTATIONS`
* Add private GCP containers / registry support for skopeo copy

## 0.30.3
* bump trivy for fixing vulnerabilities

## 0.30.2
* fixing vulnerabilities

## 0.30.1
* upgraded goreleaser to v2

## 0.30.0
* improves logs on trivy scanning failure
* adds `Error` to report output file

## 0.29.5
* fixed docker vulnerability

## 0.29.4
* fixed docker vulnerability

## 0.29.3
* add support for go workspace

## 0.29.2
* Bumped trivy version

## 0.29.1
* Bump alpine to 3.20

## 0.29.0
* update trivy version

## 0.28.13
* update dependencies

## 0.28.12
* update dependencies

## 0.28.11
* update dependencies

## 0.28.10
* update dependencies

## 0.28.9
* update dependencies

## 0.28.8
* normalize image ID and name to fix re-scan of stale images

## 0.28.7
* update trivy to 0.48.1

## 0.28.6
* Bump kubectl to 1.29.0

## 0.28.5
* Bump alpine to 3.19

## 0.28.4
* Update dependencies

## 0.28.3
* Update to go 1.21

## 0.28.2
* Update binary dependency `trivy`

## 0.28.1
* Update dependencies

## 0.28.0
* utilize the controller-utils library to correctly gather owner references

## 0.27.0
* Add support to multiple owners to images

## 0.26.2
* update dependencies

## 0.26.1
* Fix issue when allowlist is not set

## 0.26.0
* Add namespace allowlist

## 0.25.2
* Update dependencies

## 0.25.1
* update dependencies

## 0.25.0
* Fix for rolling scans when there are a lot of errors

## 0.24.10
* Allow insecure TLS for trivy using TRIVY_INSECURE env var

## 0.24.9
* update alpine and x/net

## 0.24.8
* update dependencies

## 0.24.7
* update alpine and go modules

## 0.24.6
* update dependencies

## 0.24.5
* update go modules

## 0.24.4
* update x/net and alpine

## 0.24.3
* Bugfix image recommendation that had integer short sha's as Tag 

## 0.24.2
* Add DockerImage to internal model

## 0.24.1
* update trivy

## 0.24.0
* Update trivy to version 0.34.0

## 0.23.1
* Fix a breaking change in #714 

## 0.23.0
* Use the `ImageID` to get container images instead of `Image`, to address when `ContainerStatuses.Image` only contains a SHA

## 0.22.3
* Update x/text to remove CVE

## 0.22.2
* Tune down some debug logs

## 0.22.1
* Update to go 1.19

## 0.22.0
* Build docker images for linux/arm64, and update to Go 1.19.1

## 0.21.2
* upgrade plugins on build
## 0.21.1
* Update dependencies

## 0.21.0
* Offline support

## 0.20.2
* improve image tags recommendations

## 0.20.1
* update to go 1.18 and update packages

## 0.20.0
* Fixing failing tags fetch from `quay.io`

## 0.19.3
* Update alpine to remove CVE

## 0.19.2
* update versions

## 0.19.1
* Bump alpine to 3.16

## 0.19.0
* Add os/architecture information to trivy report

## 0.18.8
* update versions

## 0.18.7
* Revert trivy version

## 0.18.6
* Update packages

## 0.18.5
* Image recommendation cleanup bug fix

## 0.18.4
* Update vulnerable packages

## 0.18.3
* Fixing cleanning up image recommnendations for specific tags

## 0.18.2
* Cleanning up image recommnendations

## 0.18.1
* Trivy Logs improvement

## 0.18.0
* Rolling images bug fix

## 0.17.1
* Update alpine to remove CVE

## 0.17.0
Update Trivy to 0.24

## 0.16.4
* Getting image tags bug fix

## 0.16.3
* Fix trivy nil bug

## 0.16.2
* Add `--ignore-unfixed` flag to the env variable

## 0.16.1
* Fix go.mod `module`, and `import`s, to use plugins sub-directory.

## 0.16.0
* Searching and scanning images recommendation

## 0.15.1
* Fix trivy command parameters for 0.23.0

## 0.15.0
* Update trivy to 0.23.0
* Remove root command

## 0.14.13
* Update dependencies
## 0.14.12
* Update trivy to 0.22.0
## 0.14.11
* Update Go modules

## 0.14.10
* Bump trivy to 0.21.3
## 0.14.9
* Bump alpine to 3.15

## 0.14.8
* Bump go modules

## 0.14.7
* rebuild to fix CVEs in alpine:3.14

## 0.14.6
* Bump dependencies and rebuild

## 0.14.5
* rebuild to fix CVEs in alpine:3.14

## 0.14.4
* rebuilt to fix CVEs in alpine 3.14

## 0.14.3
* update trivy command
## 0.14.2
* update Go modules

## 0.14.1
* Update Go and modules

## 0.14.0
* Added Severity.CRITICAL to json schema
## 0.13.0
* update go dependencies

## 0.12.3
* Bump Alpine to 3.14

## 0.12.2
* Update alpine image

## 0.12.1
* Updated json schema

## 0.12.0
* Update to latest version of Trivy
* Change version number to match upstream version of Trivy

## 0.7.0
* Added container information as a separate field.

## 0.6.6
* Add log statement

## 0.6.5
* Update Trivy to 0.11.0

## 0.6.4
* Fix bug with multiple contains in one object

## 0.6.2
* Refactor codebase

## 0.6.1
* Update to Trivy 0.6.0

## 0.6.0
* Updated to the new version of the Kubernetes API

## 0.5.0
* Roll pods up to their top level owner to consolidate results.

## 0.4.0
* Initial release
