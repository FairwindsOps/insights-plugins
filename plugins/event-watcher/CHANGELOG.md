# Changelog

## 0.2.17
* Bump github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs to version v1.68.0

## 0.2.16
* Bump github.com/aws/aws-sdk-go-v2/config to version v1.32.14
* Bump github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs to version v1.67.0

## 0.2.15
* Bump 	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs to version v1.65.0

## 0.2.14
* Bump library github.com/aws/aws-sdk-go-v2/config to version v1.32.13
* Bump indirect library dependencies

## 0.2.13
* Bump library github.com/aws/aws-sdk-go-v2 to version v1.41.5
* Bump library k8s.io/api to version v0.35.3
* Bump library k8s.io/apimachinery to version v0.35.3
* Bump library k8s.io/client-go to version v0.35.3
* Bump indirect library dependencies

## 0.2.12
* Harden Alpine-based Docker images: targeted upgrades for libcrypto3, libssl3, and zlib instead of full `apk upgrade` (narrower supply-chain exposure).

## 0.2.11
* Bump library github.com/aws/aws-sdk-go-v2 to version v1.41.4
* Bump library github.com/aws/aws-sdk-go-v2/config to version v1.32.12
* Bump library github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs to version v1.64.1
* Bump indirect library dependencies

## 0.2.10
* Bump library dependencies
* Bump indirect library dependencies

## 0.2.9
* Bump library dependencies
* Bump indirect library dependencies

## 0.2.8
* Bumped to Go 1.26
* Bump library dependencies
* Bump indirect library dependencies

## 0.2.7
* Bump library k8s.io/api, k8s.io/apimachinery, k8s.io/client-go to v0.35.1
* Bump library golang.org/x/net to v0.50.0
* Bump library golang.org/x/term to v0.40.0
* Bump library golang.org/x/text to v0.34.0
* Bump indirect library dependencies

## 0.2.6
* Bump library golang.org/x/oauth2 to v0.35.0
* Bump library golang.org/x/sys to v0.41.0
* Bump library sigs.k8s.io/structured-merge-diff/v6 to v6.3.2
* Bump indirect library dependencies

## 0.2.5
* Bump library dependencies
* Bump indirect library dependencies

## 0.2.4
* Bump indirect library dependencies

## 0.2.3
* Bumped all libs

## 0.2.2
* Bump github.com/aws/aws-sdk-go-v2 to v1.41.1

## 0.2.1
* Bump library dependencies

## 0.2.0
* Bump k8s api libraries to 0.35.0

## 0.1.26
* Bump library dependencies

## 0.1.25
* Bump library dependencies

## 0.1.24
* Bug fixes

## 0.1.23
* Fix parsing image validation policy name

## 0.1.22
* Added support to more policies Audit

## 0.1.21
* Bumped to go 1.25.5

## 0.1.20
* Support to ImageValidatingPolicy

## 0.1.19
* Support to NamespacedValidatingPolicy

## 0.1.18
* Fix event poll interval

## 0.1.17
* Fix nil when starting logs

## 0.1.16
* Added option to unlimited buffer size

## 0.1.15
* Added ValidatingPolicy as kyverno kind

## 0.1.14
* Add support to Blocked and Audit type Policy

## 0.1.13
* Add support to Audit only for ClusterPolicy

## 0.1.12
* Add support to Audit only validationg admission policy

## 0.1.11
* Bumped go for fixing vulnerabilities

## 0.1.10
* Improve parsing blocked field

## 0.1.9
* Handling validatingadmissionpolicy

## 0.1.8
* Code refactoring

## 0.1.7
* Added missing parameters

## 0.1.6
* Added big cache

## 0.1.5
* Added support to validating policy violation

## 0.1.4
* Improving logic to identify policy violation

## 0.1.3
* Improving parsing cloud watch logs

## 0.1.2
* Fix violation ID

## 0.1.1
* Fix violation event message

## 0.1.0
* Initial release
