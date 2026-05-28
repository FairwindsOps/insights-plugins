# Changelog

## Unreleased

* Composite verification framework: multiple `IMAGE_TRUST_MODES` with OR policy (`IMAGE_TRUST_MODE_POLICY=any`).
* Config for trusted public keys (`IMAGE_TRUST_PUBLIC_KEY_PATHS`, `IMAGE_TRUST_PUBLIC_KEY_DIR`) and `verifiedBy` report field.
* `cosign-key` verifier using trusted public keys (`IMAGE_TRUST_PUBLIC_KEY_PATHS` / `IMAGE_TRUST_PUBLIC_KEY_DIR`).

## 0.1.0
* Initial `image-trust` plugin: image discovery, Cosign keyless verification, allowlists, and Insights report upload.
* On-demand job runner support for report type `image-trust`.
