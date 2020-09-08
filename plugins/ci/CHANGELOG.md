# Changelog
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
