# Changelog

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
