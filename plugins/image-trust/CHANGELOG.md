# Changelog

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
