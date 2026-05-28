# Changelog

## 0.2.0

* Composite verification: multiple `IMAGE_TRUST_MODES` with OR policy (`IMAGE_TRUST_MODE_POLICY=any`, default).
* `cosign-key` verification via trusted public keys (`IMAGE_TRUST_PUBLIC_KEY_PATHS`, `IMAGE_TRUST_PUBLIC_KEY_DIR`, optional `IMAGE_TRUST_IGNORE_TLOG`).
* Report field `verifiedBy` records which mode satisfied trust.
* README: private registry credentials and public key Secret mount examples.
* `REGISTRY_DOCKER_CONFIG_PATH` for multi-registry auth via Docker `config.json` (`DOCKER_CONFIG`).

## 0.1.0
* Initial `image-trust` plugin: image discovery, Cosign keyless verification, allowlists, and Insights report upload.
* On-demand job runner support for report type `image-trust`.
