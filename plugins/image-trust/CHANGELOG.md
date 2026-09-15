# Changelog

## 0.1.13
* Rebuild Alpine runtime image to pick up libssl3/libcrypto3 3.5.8-r0 (CVE-2026-63072, CVE-2026-63073, CVE-2026-63074, CVE-2026-63075, CVE-2026-63076, CVE-2026-14456, CVE-2026-14457, CVE-2026-18798, CVE-2026-54874, CVE-2026-75803).
* Bump library github.com/google/go-containerregistry to v0.22.1
* Bump library github.com/sirupsen/logrus to v1.10.2
* Bump library github.com/stretchr/testify to v1.12.1
* Bump library k8s.io/api to v0.37.0
* Bump library k8s.io/apimachinery to v0.37.0
* Bump library k8s.io/client-go to v0.37.0
* Bump library sigs.k8s.io/controller-runtime to v0.25.0
* Bump indirect libraries dependencies

## 0.1.12
* Only push the GAR version tag when it has not been published, since GAR tags are immutable

## 0.1.11
* Bump dependencies

## 0.1.10
* Document GAR as preferred image registry

## 0.1.9
* Also publish image to Google Artifact Registry (`us-docker.pkg.dev/fairwinds-ops/oss`)

## 0.1.8
* Bump library k8s.io/api to v0.36.3
* Bump library k8s.io/apimachinery to v0.36.3
* Bump library k8s.io/client-go to v0.36.3
* Bump indirect libraries dependencies

## 0.1.7
* Bump dependencies

## 0.1.6
* Build with Go 1.26.4 (stdlib CVE-2026-42504, CVE-2026-42507, CVE-2026-27145) via module `go` version and `GOTOOLCHAIN=go1.26.4` in release builds.

## 0.1.5
* Bump library github.com/google/go-containerregistry to v0.21.7
* Bump indirect libraries dependencies

## 0.1.4
* Bump dependencies

## 0.1.3
* Bump library k8s.io/api to v0.36.2
* Bump library k8s.io/apimachinery to v0.36.2
* Bump library k8s.io/client-go to v0.36.2

## 0.1.2
* Bump dependencies

## 0.1.1
* Build with Go 1.26.3 (stdlib CVE-2026-42501, CVE-2026-39825, CVE-2026-39826, CVE-2026-39823) via module `go` version and `GOTOOLCHAIN=go1.26.3` in release builds.

## 0.1.0
* Initial `image-trust` plugin: workload image discovery (controllers, orphan pods, active Jobs), digest resolution, and Insights report upload (`/data/image-trust`).
* Cosign verification modes: `cosign-keyless`, `cosign-key`, and composite `IMAGE_TRUST_MODE_POLICY` (`any` / `all`).
* Keyless trust policy (`IMAGE_TRUST_TRUSTED_ISSUERS`, `IMAGE_TRUST_TRUSTED_SUBJECTS`, `IMAGE_TRUST_TRUSTED_SUBJECT_REGEXPS`).
* Keyed verification with mounted public keys, remote refs, and KMS (`IMAGE_TRUST_PUBLIC_KEY_*`, `IMAGE_TRUST_IGNORE_TLOG`).
* Attestation modes (`cosign-attestation-keyless`, `cosign-attestation-key`) with `IMAGE_TRUST_ATTESTATION_TYPES`.
* Allowlists (image, registry, signer), verification retries, and transient-error backoff.
* Private registry support: multi-registry docker config (`IMAGE_TRUST_REGISTRY_AUTHS`), mirrors, and per-registry TLS.
* Self-hosted / air-gapped Sigstore env forwarding (`IMAGE_TRUST_SIGSTORE_ENV_FILE`).
* On-demand job runner support for report type `image-trust`.
* JSON results schema (`results.schema`) and kind-based smoke test.
