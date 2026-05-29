# image-trust local smoke test (kind)

Exercises discovery and cosign verification against a kind cluster using **public images only** (no signing in this test).

| Deployment | Image(s) | Mode | Expected status |
|------------|----------|------|-----------------|
| `verified` | `cgr.dev/chainguard/busybox@sha256:972a590ee3f5f636860c5a9e40121ce30ad07deff5c91a035469b0e70c3a4dcf` | keyless | `verified` |
| `keyed-verified` | `us-docker.pkg.dev/fairwinds-ops/oss/polaris:v10.2.0` | keyed (`cosign-key`) | `verified` |
| `untrusted` | `gcr.io/projectsigstore/cosign:v2.4.0` (init; Sigstore signer, not Chainguard release workflow) + `docker.io/library/busybox:1.36` (main) | keyless | `signed_untrusted` for cosign; `unsigned` for busybox |
| `unsigned` | `docker.io/library/busybox:1.36` | — | `unsigned` |

**Keyless** trust policy targets the Chainguard Images release workflow.

**Keyed** verification fetches the Fairwinds OSS release public key from [`https://artifacts.fairwinds.com/cosign-p256.pub`](https://artifacts.fairwinds.com/cosign-p256.pub) via `IMAGE_TRUST_PUBLIC_KEY_REFS` (full URL appears in `signer.keyRef`). Polaris v10.2.0+ documents image verification in [FairwindsOps/polaris releases](https://github.com/FairwindsOps/polaris/releases).

The `untrusted` deployment exercises two images in one pod; expect three ActionItems total (one `unsigned` for `unsigned`, one `unsigned` + one `signed_untrusted` for `untrusted`).

## Prerequisites

- kind cluster with working `kubectl` context
- Docker
- `go`, `jq`
- Outbound network (registries + Sigstore for keyless)

## Quick start

```bash
cd plugins/image-trust/smoke
chmod +x setup.sh run.sh assert.sh

./setup.sh    # deploy workloads to kind (run.sh runs this automatically if missing)
./run.sh      # build plugin image, run report (scoped to image-trust-smoke)
./assert.sh   # check output/image-trust.json
```

Report path: `plugins/image-trust/output/image-trust.json`

## Configuration

Copy `env.example` to `env` and adjust if needed:

```bash
cp env.example env
```

Default `env.example` enables both modes:

- `IMAGE_TRUST_MODES=cosign-keyless,cosign-key`
- `IMAGE_TRUST_PUBLIC_KEY_REFS=https://artifacts.fairwinds.com/cosign-p256.pub`
- `IMAGE_TRUST_IGNORE_TLOG=true` (Fairwinds keyed images are not in Rekor)

### Optional: private registry

For private images, set `REGISTRY_USER` / `REGISTRY_PASSWORD_FILE` or `REGISTRY_DOCKER_CONFIG_PATH` in `env` (see plugin README). To reuse namespace pull credentials, set `IMAGE_TRUST_USE_IMAGE_PULL_SECRETS=true` (requires RBAC to list secrets in scoped namespaces).

## Cleanup

```bash
kubectl delete namespace image-trust-smoke
```

## Troubleshooting

- **`unknown` status** — Pod `imageID` may lack a digest; wait for rollout and re-run `./run.sh`.
- **`signed_untrusted` for Chainguard image** — Trust env does not match; compare with [Chainguard verify docs](https://edu.chainguard.dev/chainguard/chainguard-images/how-to-use/verifying-chainguard-images-and-metadata-signatures-with-cosign/).
- **`unsigned` for polaris** — Ensure polaris is v10.2.0+ and the plugin can reach `https://artifacts.fairwinds.com/cosign-p256.pub`. Confirm manually: `cosign verify us-docker.pkg.dev/fairwinds-ops/oss/polaris:v10.2.0 --key https://artifacts.fairwinds.com/cosign-p256.pub --insecure-ignore-tlog`.
- **`verification_error`** — Registry or Sigstore unreachable from the plugin container; try `--network host` (already set in `run.sh`).
