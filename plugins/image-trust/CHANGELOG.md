# Changelog

## 0.5.1

* Report field `attestationType` when attestation verification succeeds.
* `IMAGE_TRUST_ATTESTATIONS_ENABLED` auto-appends attestation modes matching enabled signature modes (or when attestation types are configured).
* Attestation auto-append follows signature mode parity only (public keys or OIDC policy alone no longer enable extra attestation modes).
* `modePolicy: all` merges attestation metadata from all successful verifiers regardless of order.
* Config validation rejects explicit attestation modes without `IMAGE_TRUST_ATTESTATION_TYPES`.

## 0.5.0

* Cosign attestation verification modes: `cosign-attestation-keyless`, `cosign-attestation-key` (`IMAGE_TRUST_ATTESTATION_TYPES`).
* Supports predicate types such as `slsaprovenance1`, `spdxjson`, and `cyclonedx`.

## 0.4.0

* Multi-registry auth via `IMAGE_TRUST_REGISTRY_AUTHS` / `_FILE` (merged into docker config; no passwords on cosign CLI).
* Registry mirror mapping (`IMAGE_TRUST_REGISTRY_MIRRORS`) for pull-through caches.
* Per-registry TLS (`IMAGE_TRUST_REGISTRY_CERT_DIRS`) with merged CA bundle for cosign and digest resolution.
* Digest lookup failures surface as `verification_error` with `digestResolveError` in the report.
* Expanded discovery: orphan running pods and active Kubernetes Jobs.
* Private Sigstore env passthrough (`IMAGE_TRUST_SIGSTORE_ENV_FILE` + well-known variables).
* Report exports `candidateSigners`; schema updated.
* Configurable verify retry backoff and jitter (`IMAGE_TRUST_VERIFY_RETRY_BACKOFF_SECONDS`, `IMAGE_TRUST_VERIFY_RETRY_JITTER`).
* RBAC manifest and chart integration guide (`deploy/rbac.yaml`, `CHART_INTEGRATION.md`).

## 0.3.0

* Resolve tag-only images to registry digests before verification (`IMAGE_TRUST_RESOLVE_DIGESTS`, default on).
* Merge workload `imagePullSecrets` for registry auth (`IMAGE_TRUST_USE_IMAGE_PULL_SECRETS`).
* Remote public key URIs for `cosign-key` (`IMAGE_TRUST_PUBLIC_KEY_REFS`, KMS URLs).
* `IMAGE_TRUST_MODE_POLICY=all` requires every configured mode to verify.
* Retry transient verification failures (`IMAGE_TRUST_VERIFY_RETRIES`, default 3).
* README limitations section and trust-policy AND semantics documented.

## 0.2.0

* Composite verification: multiple `IMAGE_TRUST_MODES` with OR policy (`IMAGE_TRUST_MODE_POLICY=any`, default).
* `cosign-key` verification via trusted public keys (`IMAGE_TRUST_PUBLIC_KEY_PATHS`, `IMAGE_TRUST_PUBLIC_KEY_DIR`, optional `IMAGE_TRUST_IGNORE_TLOG`).
* Report field `verifiedBy` records which mode satisfied trust.
* README: private registry credentials and public key Secret mount examples.
* `REGISTRY_DOCKER_CONFIG_PATH` for multi-registry auth via Docker `config.json` (`DOCKER_CONFIG`).

## 0.1.0
* Initial `image-trust` plugin: image discovery, Cosign keyless verification, allowlists, and Insights report upload.
* On-demand job runner support for report type `image-trust`.
