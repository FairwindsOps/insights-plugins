# Changelog

## 6.1.7
* Bumped libs version

## 6.1.6
* Bump trivy to 0.67.0

## 6.1.5
* Bump trivy to 0.66.0

## 6.1.4
* Bumped helm version

## 6.1.3
* Bumped go to 1.24.6 for fixing vulnerability

## 6.1.2
* Bump trivy to 0.65.0

## 6.1.1
* Bumping helm

## 6.1.0
* Bumped polaris version to 10.0.0

## 6.0.0
* Remove support to OPA V1

## 5.9.12
* Bump pluto to 5.21.9

## 5.9.11
* Bump trivy to 0.64.1

## 5.9.10
* Fixing vulnerabilities

## 5.9.9
* Fixing vulnerabilities

## 5.9.8
* Remove tfsec support
* Update libraries

## 5.9.7
* bumped alpine to 3.22

## 5.9.6
* Upgaded trivy to 0.62.1

## 5.9.5
* bumped tfsec

## 5.9.4
* bumped libs

## 5.9.3
* bumped libs

## 5.9.2
* upgaded trivy to 0.61.0

## 5.9.1
* Fixed trivy vulnerability

## 5.9.0
* Added annotations to scan workloads

## 5.8.1
* Support to Rego V0 and V1 libs

## 5.8.0
* Support to Rego V1

## 5.7.20
* Fixing vulnerabilities

## 5.7.19
* Fixed helm vulnerability

## 5.7.18
* Fixed trivy vulnerability

## 5.7.17
* Fixed tfsec vulnerability

## 5.7.16
* Fixing vulnerabilities

## 5.7.15
* Fixing vulnerabilities

## 5.7.14
* bumped alpine to 3.21

## 5.7.13
* bumped libs

## 5.7.12
- bumped trivy to v0.57.1

## 5.7.11
* bumped polaris to 9.6.0

## 5.7.10
- fix trivy db / java-db downloading

## 5.7.9
- bumped trivy to v0.57.0

## 5.7.8
- Use `RemoveTokensAndPassword` function from trivy
- Add trivy OCI repositories fallback

## 5.7.7
- bumped trivy to v0.56.2

## 5.7.6
- bumped tfsec to 1.28.11 for fixing vulnerabilities

## 5.7.5
- Fixing vulnerabilities

## 5.7.4
- Bumped pluto to 5.20.2

## 5.7.3
- upgrading due to trivy API change

## 5.7.2
- fixed helm vulnerability

## 5.7.1
- fixed docker vulnerability

## 5.7.0
- Add support for OPA custom libs

## 5.6.2
- fixed docker vulnerability

## 5.6.1
- add support for go workspace

## 5.6.0
- Add support for scan-workloads labels

## 5.5.9
- Bumped trivy version

## 5.5.8
- Fixed CI vulnerability

## 5.5.7
- Bump alpine to 3.20

## 5.5.6
- bumped versions

## 5.5.5
- bumped pluto to 5.19.4

## 5.5.4
- bumped pluto to 5.19.3

## 5.5.3
- bumped polaris to 9.0.1

## 5.5.2
- bumped tfsec to 1.28.6 for fixing vulnerabilities

## 5.5.1

- update dependencies

## 5.5.0

- update tfsec and trivy version

## 5.4.5

- update dependencies

## 5.4.4

- update dependencies

## 5.4.3

- update dependencies

## 5.4.2

- update dependencies

## 5.4.1

- update dependencies

## 5.4.0

- Added tfsec custom checks

## 5.3.4

- Fix image owners matching logic

## 5.3.3

- Trim spaces from masterBranch before using it

## 5.3.2

- Only skip failed files instead of halting the process

## 5.3.1

- Print soft-errors to output to increase error awareness

## 5.3.0

- Added files that were modified to CI scan response

## 5.2.9

- update trivy to 0.48.1

## 5.2.8

- Bump alpine to 3.19

## 5.2.7

- Ignore `setExitCode` when running on auto-scan mode

## 5.2.6

- Update dependencies

## 5.2.5

- Fix removing `.git` from repository name

## 5.2.4

- Update to go 1.21

## 5.2.3

- Update binary dependency `trivy`

## 5.2.2

- Update dependencies

## 5.2.1

- Update dependencies

## 5.2.0

- Add `reports.goldilocks.enabled` support (default `true`)
- Add `reports.prometheus-metrics.enabled` support (default `true`)

## 5.1.3

- Add warning message and prevent panic when we find a podSpec with no containers

## 5.1.2

- Bump polaris version to 8.2.3

## 5.1.1

- Update go libraries
- Update trivy/opa version

## 5.1.0

- Update dependencies (polaris 8.0.0)

## 5.0.4

- Update dependencies

## 5.0.3

- Update dependencies

## 5.0.2

- update dependencies

## 5.0.1

- Support for insecure TLS override in uploader

## 5.0.0

- Fixes bug where relative path were not preserved on filename field for yaml manifest files.

## 4.2.10

- update alpine and x/net

## 4.2.9

- Restore command standard-error being returned and reflected in CI logs and scan-error report action items, from PR #754.

## 4.2.8

- Fix STDOUT parsing

## 4.2.7

- update dependencies

## 4.2.6

- update alpine and go modules

## 4.2.5

- Clarify the log message when there have been no tfsec findings after processing all terraform paths.

## 4.2.4

- Fix removal of the repository path from tfsec result file names, when said result is for a Terraform module. THis bug caused these file names to begin with `/app/repository/{repository name}`.
- Log the version of the CI plugin.

## 4.2.3

- Revert 4.2.2

## 4.2.2

- Update pluto from 5.11.2 to 5.12.0
- Update Polaris from 7.2.1 to 7.3.0
- Update Helm from 3.10.3 to 3.11.0

## 4.2.1

- update dependencies

## 4.2.0

- CI scanning will continue when an error is encountered, such as templating a Helm chart into Kubernetes manifests. These errors will be reflected as Insights action items, in a new `ScanErrors` report type.

## 4.1.5

- skip downloading in-container `images.docker` images that has env. variables on their names

## 4.1.4

- update go modules

## 4.1.3

- Fixes when using `helm.values` causes tmp filepath to get mangled

## 4.1.2

- Fixes missing image info (name and owner name) when the download of `docker.images` are done inside the CI plugin execution

## 4.1.1

- update x/net and alpine

## 4.1.0

- Add support for configuring reports when using auto-discovery via `REPORTS_CONFIG` env var

## 4.0.0

- Enable the tfsec report by default. If `terraform -> paths` are specified, they will be scanned unless `reports -> tfsec -> enabled` is explicitly set to `false` in fairwinds-insights.yaml.

## 3.4.0

- Support for private images (REGISTRY_CREDENTIALS)

## 3.3.0

- Support `images.docker` download images inside the plugin

## 3.2.1

- update trivy

## 3.2.0

- Add alternative GIT commands to fetch masterHash
- Make some GIT commands optional (masterHash, commitMessage, branch and origin)
- Add CI_RUNNER env. var support
- Add hint logs based on CI runner

## 3.1.0

- Update tfsec, pluto, and polaris to adress additional `x/text` and `x/net` CVEs
- Bump Helm to 3.10.2

## 3.0.0

- Add Terraform scanning via a tfsec report

## 2.4.1

- Temporarily revert terraform scanning

## 2.4.0

- Add Terraform scanning via a tfsec report

## 2.3.0

- Update trivy to version 0.34.0

## 2.2.4

- Update x/text to remove CVE

## 2.2.3

- Update dependencies

## 2.2.2

- Update to go 1.19

## 2.2.1

- Update versions

## 2.2.0

- Build docker images for linux/arm64, and update to Go 1.19.1

## 2.1.12

- Improves logging to show k8s and helm files

## 2.1.11

- Fix `helm template` command in some environments

## 2.1.10

- Fix leaking access token in std out.

## 2.1.9

- upgrade plugins on build

## 2.1.8

- Fix for missing fields in container manifests

## 2.1.7

- Update dependencies

## 2.1.6

- Fix OPA panic if `kind` field is missing

## 2.1.5

- update packages

## 2.1.4

- update packages

## 2.1.3

- Fix for git 2.35.2

## 2.1.2

- support HPA v2beta1 in OPA checks

## 2.1.1

- update to go 1.18 and update packages

## 2.1.0

- update Trivy plugin

## 2.0.3

- Update alpine to remove CVE

## 2.0.2

- Add debug info

## 2.0.1

- update versions

## 2.0.0

- updated CI NewActionItemThreshold default to -1

## 1.6.2

- Fix auto-detection on resolving non-kubernetes manifests.

## 1.6.1

- Bump alpine to 3.16

## 1.6.0

- Add `ValuesFiles` to fairwinds-insights.yaml, allowing specification of multiple Helm values files.
- Allow both Helm values files and inline fairwinds-insights.yaml values to be used. The inline values override those from values files.

## 1.5.8

- update versions

## 1.5.7

- update versions

## 1.5.6

- Add option to add more skopeo arguments through `SKOPEO_ARGS` environment variable

## 1.5.5

- Fix trivy scan output location

## 1.5.4

- Revert trivy version

## 1.5.3

- Update packages

## 1.5.2

- Image scannning update

## 1.5.1

- Update vulnerable packages

## 1.5.0

- Trivy no longer downloads images

## 1.4.2

- Update alpine to remove CVE

## 1.4.1

- Obtain the OPA version from its Go package when submitting an OPA report (commit cd93f76).

## 1.4.0

- Update Trivy to 0.24

## 1.3.3

- Fix go.mod.

## 1.3.2

- Fix trivy `image.ScanImageFile` arguments

## 1.3.1

- Fix go.mod `module`, and `import`s, to use plugins sub-directory.

## 1.3.0

- Process v2 CustomChecks, which no longer have an Instance accompanying the rego policy.
- Debug output can be enabled by setting the `LOGRUS_LEVEL` environment variable to `debug`.
- Processing of checks will now continue when there has been a failure, to collect and output all failure conditions. Multiple errors may be reflected in plugin output.

## 1.2.3

- Updated libs

## 1.2.2

- Fix trivy command parameters on 0.23.0

## 1.2.1

- Updated trivy version to 0.23.0
- Drop root command

## 1.2.0

- Adds auto config. file generation by scanning the repository files

## 1.1.1

- Fix reading helm `valuesFile` and `fluxFile` when on cloned repo context
- Fix internal `baseFolder` when not in cloned repo context

## 1.1.0

- Add an `insightsinfo` function to make Insights information available in rego.

## 1.0.0

- Update plugin version

## 0.15.1

- Run apk update

## 0.15.0

- Support for external git repository

## 0.14.2

- Update dependencies

## 0.14.1

- Update OPA for removed CRD.

## 0.14.0

- Update Polaris to version 5.0.0
- Update Pluto to version v5.3.2

## 0.13.7

- Updated trivy version to 0.22.0

## 0.13.6

- Adds the HTTP body to the error to provide better error messages

## 0.13.5

- Update Go modules

## 0.13.4

- Updated trivy version

## 0.13.3

- Fix panic for missing sha in the image

## 0.13.2

- Bump alpine to 3.15

## 0.13.1

- Bump go modules

## 0.13.0

- Added environment variable for git informations.

## 0.12.1

- rebuild to fix CVEs in alpine:3.14

## 0.12.0

- Add helm `fluxFile` and `version` support

## 0.11.0

- Add helm remote chart functionality

## 0.10.13

- Bump dependencies and rebuild

## 0.10.12

- Handle type conversion errors for resource metadata

## 0.10.11

- rebuild to fix CVEs in alpine:3.14

## 0.10.10

- rebuilt to fix CVEs in alpine 3.14

## 0.10.9

- update trivy version

## 0.10.8

- update Go modules

## 0.10.7

- Improve error messages
- Add missing error checks

## 0.10.6

- Add SHA for docker images

## 0.10.5

- Add option to skip images contained in manifests when running trivy

## 0.10.4

- Add some debug logs

## 0.10.3

- Handle error in walkpath

## 0.10.2

- Update Go and modules

## 0.10.1

- Improve error handling in CI's git fetch info process

## 0.10.0

- update go dependencies

## 0.9.2

- Fix bug in Trivy to allow namespace to be sent up.

## 0.9.1

- Bump Alpine to 3.14

## 0.9.0

- Added configuration options to disable individual reports

## 0.8.5

- Fix `Options.TempFolder` default destination

## 0.8.4

- Update alpine image

## 0.8.3

- Fix workload names

## 0.8.2

- Fix helm file name by replacing the release-name prefix.

## 0.8.1

- Dedupe Trivy scans

## 0.8.0

- Improved logging and output

## 0.7.2

- Respect mainline branch specified in config.

## 0.7.1

- update Trivy

## 0.7.0

- Add commit messages to scan

## 0.6.0

- Start sending fairwinds-insights.yaml to backend

## 0.5.0

- Add OPA as another check
- Add Pluto as another check

## 0.4.10

- Strip tags from manifest free images

## 0.4.9

- Added containers to workloads report
- Add container name to Trivy results

## 0.4.8

- Add log statement to Trivy

## 0.4.7

- Update Trivy to 0.11.0

## 0.4.6

- Added name to images that aren't in manifest

## 0.4.5

- Remove "**\*\***.com:" prefix and ".git" suffix from default repo name

## 0.4.4

- Update CHANGELOG

## 0.4.3

- Made `repositoryName` optional

## 0.4.2

- Fixed a bug in error output

## 0.4.0

- created a separate `RunCommand` that doesn't have trivy-specific logic
- started logging stdout/stderr directly instead of through logrus, to preserve newlines
- fixed formatting on message
- remove `panic`s
- push helm values to file instead of using `--set`
- change output message
- set config defaults

## 0.3.0

- Updating Polaris version from 0.6 to 1.1

## 0.2.0

- New config format
- Send Kubernetes Resources to be saved
- Base results based on new action items instead of "Score"

## 0.1.1

- Process helm templates

## 0.1.0

- Initial release
