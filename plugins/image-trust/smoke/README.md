# image-trust local smoke test (kind)

Exercises discovery and cosign verification against a kind cluster using **public images only** (no signing).

| Deployment | Image | Expected status |
|------------|-------|-----------------|
| `verified` | `cgr.dev/chainguard/busybox:latest` | `verified` |
| `untrusted` | `gcr.io/projectsigstore/cosign:v2.4.0` (Sigstore signer, not Chainguard release workflow) | `signed_untrusted` |
| `unsigned` | `docker.io/library/busybox:1.36` | `unsigned` |

Trust policy trusts the Chainguard Images release workflow. The Sigstore cosign image is signed by a different identity.

## Prerequisites

- kind cluster with working `kubectl` context
- Docker
- `go`, `jq`
- Outbound network (registries + Sigstore)

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

### Optional: keyed verification

To exercise `cosign-key` locally, add a vendor public key under `../testdata/keys/` and set in `env`:

```bash
export IMAGE_TRUST_MODES='cosign-keyless,cosign-key'
export IMAGE_TRUST_PUBLIC_KEY_DIR='../testdata/keys'
```

Mount the directory in `run.sh` if you extend the docker invocation with `-v "$(pwd)/../testdata/keys:/etc/image-trust/keys:ro"` and `IMAGE_TRUST_PUBLIC_KEY_DIR=/etc/image-trust/keys`.

### Optional: private registry

For private images, set `REGISTRY_USER` / `REGISTRY_PASSWORD_FILE` or `REGISTRY_DOCKER_CONFIG_PATH` in `env` (see plugin README).

## Cleanup

```bash
kubectl delete namespace image-trust-smoke
```

## Troubleshooting

- **`unknown` status** — Pod `imageID` may lack a digest; wait for rollout and re-run `./run.sh`.
- **`signed_untrusted` for Chainguard image** — Trust env does not match; compare with [Chainguard verify docs](https://edu.chainguard.dev/chainguard/chainguard-images/how-to-use/verifying-chainguard-images-and-metadata-signatures-with-cosign/).
- **`verification_error`** — Registry or Sigstore unreachable from the plugin container; try `--network host` (already set in `run.sh`).
