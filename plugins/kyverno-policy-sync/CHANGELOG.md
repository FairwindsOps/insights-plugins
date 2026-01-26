# Changelog

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
