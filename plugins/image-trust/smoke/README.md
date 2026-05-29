# image-trust local smoke test (kind)

Exercises discovery and cosign verification against a kind cluster using **public images only** (no signing in this test).

| Deployment | Image | Mode | Expected status |
|------------|-------|------|-----------------|
| `verified` | `cgr.dev/chainguard/busybox:latest` | keyless | `verified` |
| `keyed-verified` | `us-docker.pkg.dev/fairwinds-ops/oss/polaris:v10.2.0` | keyed (`cosign-key`) | `verified` |
| `untrusted` | `gcr.io/projectsigstore/cosign:v2.4.0` (Sigstore signer, not Chainguard release workflow) | keyless | `signed_untrusted` |
| `unsigned` | `docker.io/library/busybox:1.36` | — | `unsigned` |

**Keyless** trust policy targets the Chainguard Images release workflow.

**Keyed** verification trusts the Fairwinds OSS release public key [`cosign-p256.pub`](https://artifacts.fairwinds.com/cosign-p256.pub), committed as `testdata/keys/fairwinds-cosign-p256.pub`. Polaris v10.2.0+ documents image verification in [FairwindsOps/polaris releases](https://github.com/FairwindsOps/polaris/releases).

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
- `IMAGE_TRUST_PUBLIC_KEY_DIR=../testdata/keys` (mounted read-only in `run.sh`)
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
- **`unsigned` for polaris** — Ensure polaris is v10.2.0+ and `fairwinds-cosign-p256.pub` is in `testdata/keys/`. Confirm manually: `cosign verify us-docker.pkg.dev/fairwinds-ops/oss/polaris:v10.2.0 --key testdata/keys/fairwinds-cosign-p256.pub`.
- **`verification_error`** — Registry or Sigstore unreachable from the plugin container; try `--network host` (already set in `run.sh`).
