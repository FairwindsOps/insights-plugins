# Changelog

## 0.12.1
* rebuild to fix CVEs in alpine:3.14

## 0.12.0
* Add helm `fluxFile` and `version` support
## 0.11.0
* Add helm remote chart functionality
## 0.10.13
* Bump dependencies and rebuild
## 0.10.12
* Handle type conversion errors for resource metadata

## 0.10.11
* rebuild to fix CVEs in alpine:3.14

## 0.10.10
* rebuilt to fix CVEs in alpine 3.14

## 0.10.9
* update trivy version
## 0.10.8
* update Go modules
## 0.10.7
* Improve error messages 
* Add missing error checks

## 0.10.6
* Add SHA for docker images

## 0.10.5
* Add option to skip images contained in manifests when running trivy

## 0.10.4
* Add some debug logs

## 0.10.3
* Handle error in walkpath

## 0.10.2
* Update Go and modules

## 0.10.1
* Improve error handling in CI's git fetch info process

## 0.10.0
* update go dependencies

## 0.9.2
* Fix bug in Trivy to allow namespace to be sent up.

## 0.9.1
* Bump Alpine to 3.14

## 0.9.0

* Added configuration options to disable individual reports

## 0.8.5

* Fix `Options.TempFolder`  default destination

## 0.8.4
* Update alpine image

## 0.8.3
* Fix workload names

## 0.8.2

* Fix helm file name by replacing the release-name prefix.

## 0.8.1

* Dedupe Trivy scans

## 0.8.0

* Improved logging and output

## 0.7.2

* Respect mainline branch specified in config.

## 0.7.1
* update Trivy

## 0.7.0
* Add commit messages to scan

## 0.6.0
* Start sending fairwinds-insights.yaml to backend

## 0.5.0
* Add OPA as another check
* Add Pluto as another check

## 0.4.10
* Strip tags from manifest free images

## 0.4.9
* Added containers to workloads report
* Add container name to Trivy results

## 0.4.8
* Add log statement to Trivy

## 0.4.7
* Update Trivy to 0.11.0

## 0.4.6
* Added name to images that aren't in manifest

## 0.4.5
* Remove "******.com:" prefix and ".git" suffix from default repo name

## 0.4.4
* Update CHANGELOG

## 0.4.3
* Made `repositoryName` optional

## 0.4.2
* Fixed a bug in error output

## 0.4.0
* created a separate `RunCommand` that doesn't have trivy-specific logic
* started logging stdout/stderr directly instead of through logrus, to preserve newlines
* fixed formatting on message
* remove `panic`s
* push helm values to file instead of using `--set`
* change output message
* set config defaults

## 0.3.0

* Updating Polaris version from 0.6 to 1.1

## 0.2.0

* New config format
* Send Kubernetes Resources to be saved
* Base results based on new action items instead of "Score"

## 0.1.1

* Process helm templates

## 0.1.0

* Initial release
