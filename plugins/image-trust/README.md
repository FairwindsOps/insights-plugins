# Image Trust

Reports image trust for container images running in a cluster.

The plugin discovers images used by workloads (including init and ephemeral containers), resolves tag-only references to digests when possible, verifies signatures with Cosign (keyless and/or static public keys), applies trust policy and allowlists, and uploads a report to Insights at `/data/image-trust`.

For background on how Cosign stores signatures in the registry, keyed vs keyless signing, and what image-trust checks during verification, see [docs/COSIGN.md](docs/COSIGN.md).

## Report output

- **images**: trust result for every discovered image (verified, unsigned, signed_untrusted, verification_error, unknown)
- **summary**: counts by status
- **policy**: snapshot of exemption allowlists configured at scan time (`IMAGE_TRUST_IMAGE_ALLOWLIST`, `IMAGE_TRUST_REGISTRY_ALLOWLIST`, `IMAGE_TRUST_SIGNER_ALLOWLIST`); pair with per-image `allowlisted` / `allowlistReason` to see which images matched
- **ActionItems**: findings for non-compliant images (allowlisted images are listed but do not generate findings)

Output file: `/output/image-trust.json` (written atomically from `/output/image-trust-temp.json`, same pattern as `kyverno` and `rbac-reporter`; the `insights-uploader` sidecar watches this path).

## Configuration

Namespace scope:

- `IMAGE_TRUST_NAMESPACE_ALLOWLIST`
- `IMAGE_TRUST_NAMESPACE_BLOCKLIST`

Verification:

- `IMAGE_TRUST_MODES` — comma-separated modes (default `cosign-keyless`). Supported: `cosign-keyless`, `cosign-key`, `cosign-attestation-keyless`, `cosign-attestation-key`. When multiple modes are set, an image is **verified** if **any** mode succeeds (`IMAGE_TRUST_MODE_POLICY=any`, default), or **all** modes succeed when `IMAGE_TRUST_MODE_POLICY=all`. With `any`, the first successful mode in list order wins.
- `IMAGE_TRUST_ATTESTATION_TYPES` — predicate types for attestation modes (e.g. `slsaprovenance1`, `spdxjson`, `cyclonedx`; comma-separated). When multiple types are configured, **any one** matching type satisfies attestation verification (OR).
- `IMAGE_TRUST_ATTESTATIONS_ENABLED` — when `true` (or types are set), matching attestation modes are appended for each enabled signature mode (`cosign-keyless` → `cosign-attestation-keyless`, `cosign-key` → `cosign-attestation-key`)
- `IMAGE_TRUST_MODE_POLICY` — `any` (default) or `all`
- `IMAGE_TRUST_TRUSTED_ISSUERS` — comma-separated OIDC issuers (keyless)
- `IMAGE_TRUST_TRUSTED_SUBJECTS` — exact certificate identities (keyless)
- `IMAGE_TRUST_TRUSTED_SUBJECT_REGEXPS` — identity regexes (max 32 patterns, 512 characters each)
- `MAX_CONCURRENT_SCANS` — parallel cosign verifications (default `5`)
- `IMAGE_VERIFY_TIMEOUT_SECONDS` — per-image verify timeout (default `180`)
- `IMAGE_TRUST_VERIFY_RETRIES` — retries for transient registry/Sigstore errors (default `3`)
- `IMAGE_TRUST_VERIFY_RETRY_BACKOFF_SECONDS` — delay between retries (default `2`)
- `IMAGE_TRUST_VERIFY_RETRY_JITTER` — add random jitter to backoff (default `true`)
- `IMAGE_TRUST_RESOLVE_DIGESTS` — resolve tag-only images to digests via the registry API before verify (default `true`; set `false` to disable)

When `cosign-keyless` is enabled, at least one of `IMAGE_TRUST_TRUSTED_ISSUERS`, `IMAGE_TRUST_TRUSTED_SUBJECTS`, or `IMAGE_TRUST_TRUSTED_SUBJECT_REGEXPS` must be configured.

When both **issuer** and **subject** policy are set, a signer must match **both** (AND). Configure only issuers or only subjects if you need OR-style matching within that dimension.

When `cosign-key` is enabled:

- `IMAGE_TRUST_PUBLIC_KEY_PATHS` — comma-separated local paths to `.pub` / PEM files
- `IMAGE_TRUST_PUBLIC_KEY_REFS` — comma-separated remote key URIs (for example `https://`, `gcpkms://`, `azurekms://`, `awskms://`)
- `IMAGE_TRUST_PUBLIC_KEY_DIR` — directory of public keys (e.g. `/etc/image-trust/keys` from a mounted Secret)
- `IMAGE_TRUST_IGNORE_TLOG` — set to `true` for keyed signatures without Rekor (`cosign` `--insecure-ignore-tlog`)

Example (both modes, keys mounted from a Secret):

```yaml
env:
  - name: IMAGE_TRUST_MODES
    value: "cosign-keyless,cosign-key"
  - name: IMAGE_TRUST_PUBLIC_KEY_DIR
    value: "/etc/image-trust/keys"
volumeMounts:
  - name: image-trust-keys
    mountPath: /etc/image-trust/keys
    readOnly: true
```

Allowlists (glob patterns; findings suppressed when matched):

- `IMAGE_TRUST_IMAGE_ALLOWLIST`
- `IMAGE_TRUST_REGISTRY_ALLOWLIST`
- `IMAGE_TRUST_SIGNER_ALLOWLIST` — matches issuer, subject, or `issuer|subject`

## Private registries

Discovery uses in-cluster pod status (no registry login). **Verification** calls the registry API to fetch signatures, so the image-trust pod needs credentials that can **read** the repository (robot account or token). Configure dedicated registry credentials on the image-trust pod — workload `imagePullSecrets` are not used.

Use the same variables as the Trivy plugin:

- `REGISTRY_USER`, `REGISTRY_PASSWORD`, `REGISTRY_PASSWORD_FILE`, `REGISTRY_CERT_DIR`

Credentials are **always** written to a docker `config.json` for cosign (passwords are never passed on the command line). Prefer `REGISTRY_PASSWORD_FILE` over `REGISTRY_PASSWORD`.

**Multi-registry auth (recommended)** — `IMAGE_TRUST_REGISTRY_AUTHS` (JSON array) or `IMAGE_TRUST_REGISTRY_AUTHS_FILE`:

```json
[
  {"host": "https://index.docker.io/v1/", "username": "user", "password": "token"},
  {"host": "https://ghcr.io", "username": "user", "password": "token"}
]
```

**Registry mirrors** (signatures on upstream): `IMAGE_TRUST_REGISTRY_MIRRORS` or `IMAGE_TRUST_REGISTRY_MIRRORS_FILE` (comma-separated `mirror=upstream` pairs)

**Per-registry TLS**: `IMAGE_TRUST_REGISTRY_CERT_DIRS` or `IMAGE_TRUST_REGISTRY_CERT_DIRS_FILE` (comma-separated `host=/path` pairs)

Example (single registry via password file):

```yaml
env:
  - name: REGISTRY_USER
    value: robot
  - name: REGISTRY_PASSWORD_FILE
    value: /etc/registry/password
volumeMounts:
  - name: registry-credentials
    mountPath: /etc/registry
    readOnly: true
volumes:
  - name: registry-credentials
    secret:
      secretName: image-trust-registry
```

Legacy single-registry env vars (`REGISTRY_USER` / `REGISTRY_PASSWORD`) are merged into docker config for `IMAGE_TRUST_REGISTRY_AUTH_HOST` (default `https://index.docker.io/v1/`).

- `REGISTRY_DOCKER_CONFIG_PATH` — existing `config.json` merged with `IMAGE_TRUST_REGISTRY_AUTHS` and legacy `REGISTRY_*` env vars

Private registry verification requires outbound access to the registry and, for keyless signatures, to Sigstore services (Fulcio, Rekor, and TUF roots).

### Self-hosted or air-gapped Sigstore

- **Keyed signing:** use `cosign-key` with `IMAGE_TRUST_IGNORE_TLOG=true` and no Rekor dependency.
- **Keyless:** set `FULCIO_URL`, `REKOR_URL`, `SIGSTORE_ROOT_FILE`, etc. on the pod, or use `IMAGE_TRUST_SIGSTORE_ENV_FILE` (one `KEY=VALUE` per line). These are forwarded to cosign subprocesses.

### KMS public keys

Use `IMAGE_TRUST_PUBLIC_KEY_REFS` with cloud credentials on the pod (`GOOGLE_APPLICATION_CREDENTIALS`, `AWS_ROLE_ARN` + `AWS_WEB_IDENTITY_TOKEN_FILE`, Azure federated token env vars). See [CHART_INTEGRATION.md](CHART_INTEGRATION.md).

## Scan duration

Wall time scales with the number of unique images, configured verification modes (including attestation modes appended when enabled), and registry latency. A rough upper bound per pass is:

`(unique_images × modes_per_image / MAX_CONCURRENT_SCANS) × IMAGE_VERIFY_TIMEOUT_SECONDS`

With defaults (`MAX_CONCURRENT_SCANS=5`, `IMAGE_VERIFY_TIMEOUT_SECONDS=180`), 100 unique images and two modes can take up to ~2 hours if every check hits the per-image timeout. Increase `MAX_CONCURRENT_SCANS`, lower `IMAGE_VERIFY_TIMEOUT_SECONDS`, or scope namespaces for large clusters.

Progress is logged during verification (`processed image trust checks for N/M images`). The on-demand job runner timeout for `image-trust` is 20 minutes (same as Trivy); align CronJob `activeDeadlineSeconds` with expected cluster size.

## Limitations (Cosign-only)

| Topic | Behavior |
|-------|----------|
| Signature types | OCI Cosign signatures in the registry only (not Notary v1, GPG, or offline bundles) |
| Attestations | In-toto attestations via `cosign verify-attestation` when enabled; report includes `attestationType`; predicate **type** only (not SLSA level, builder, or SBOM content); multiple configured types are OR (any match passes); no custom Rego/CUE policy files |
| Discovery | Top-level controllers, orphan running pods (no controller owner), and active Jobs with **running** pods (not completed/historical workloads or pending pods) |
| Digest | Failed registry lookup → `verification_error` with `digestResolveError`; tag-only without lookup → `unknown` |
| Registry mirrors | Configure `IMAGE_TRUST_REGISTRY_MIRRORS` when signatures live on upstream hosts |

## Product integration (this repo)

| Component | Status |
|-----------|--------|
| Plugin binary + Docker image | `quay.io/fairwinds/image-trust` |
| Uploader datatype | `image-trust` → `/data/image-trust` |
| On-demand jobs | `image-trust` report type in `on-demand-job-runner` |
| JSON schema | `plugins/image-trust/results.schema` |

**Fairwinds Insights Agent chart** ([charts](https://github.com/FairwindsOps/charts) repo): see [CHART_INTEGRATION.md](CHART_INTEGRATION.md) and `stable/insights-agent/README.md`.

## Running locally

Build from the repository root so GoReleaser artifacts line up with other plugins:

```bash
cd plugins/image-trust
CGO_ENABLED=0 GOOS=linux GOARCH="$(go env GOARCH)" go build -o image-trust ./cmd
docker build -t fw-image-trust .
docker run --network host \
  --user 0:0 \
  -e KUBECONFIG=/kube/config \
  -e IMAGE_TRUST_TRUSTED_ISSUERS=https://token.actions.githubusercontent.com \
  -e IMAGE_TRUST_TRUSTED_SUBJECT_REGEXPS='https://github.com/my-org/.+' \
  -v "$HOME/.kube/config:/kube/config:ro" \
  -v "$(pwd)/output:/output" \
  fw-image-trust
```

The published image runs as UID `1200`; mount kubeconfig read-only and set trust policy env vars as required.

Cosign **v3.0.6** is included in the published Dockerfile with release checksum verification.

## Local smoke test (kind)

See [smoke/README.md](smoke/README.md) for a kind-based smoke test using public images only (`setup.sh`, `run.sh`, `assert.sh`). Report is written to `output/image-trust.json`.
