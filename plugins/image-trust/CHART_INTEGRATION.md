# Insights Agent chart integration

The Fairwinds Insights Agent chart lives in [FairwindsOps/charts](https://github.com/FairwindsOps/charts). Wire these values when enabling image-trust.

## RBAC

When `IMAGE_TRUST_USE_IMAGE_PULL_SECRETS=true`, the plugin loads only `kubernetes.io/dockerconfigjson` secrets referenced by discovered pod `imagePullSecrets` and the pod service account's `imagePullSecrets`. Minimum rules for that feature: `secrets` **get**, `serviceaccounts` **get**, and `namespaces` **get/list** (namespace list is used for discovery scoping).

Also grants `pods` and `jobs` **list/get** for orphan-pod and active-job discovery paths. Bind that ClusterRole to the image-trust ServiceAccount (or merge equivalent rules into your chart RBAC). Example chart fragment:

```yaml
image-trust:
  enabled: true
  useImagePullSecrets: true
```

| Resource | Verbs | Used for |
|----------|-------|----------|
| `secrets` | get | Referenced imagePullSecrets (`IMAGE_TRUST_USE_IMAGE_PULL_SECRETS`) |
| `serviceaccounts` | get | Service account imagePullSecrets for discovered pods |
| `namespaces` | get, list | Namespace scoping for discovery |
| `pods` | get, list | Orphan running pods not owned by a top-level controller |
| `jobs` | get, list | Active Jobs and their running pods |

### Pull-secret RBAC blast radius

When `useImagePullSecrets: true`, the bundled ClusterRole grants **`secrets` get cluster-wide**. The plugin only reads `kubernetes.io/dockerconfigjson` secrets referenced by discovered pod `imagePullSecrets` and service accounts, but Kubernetes RBAC cannot express that constraint — any Secret in any namespace could be read by the image-trust ServiceAccount if an attacker knows the name.

Prefer dedicated registry credentials (`IMAGE_TRUST_REGISTRY_AUTHS`, `REGISTRY_PASSWORD_FILE`) when possible and leave `useImagePullSecrets: false`. If pull-secret merge is required, treat the image-trust ServiceAccount as sensitive and restrict who can exec into the pod or read its environment.

## Registry credentials

| Chart value / env | Plugin env |
|-------------------|------------|
| `privateImages.registryAuthsSecret` | `IMAGE_TRUST_REGISTRY_AUTHS_FILE` (JSON array in mounted secret) |
| `privateImages.dockerConfigSecret` | `REGISTRY_DOCKER_CONFIG_PATH` |
| `privateImages.registryUser` / `registryPasswordSecret` | `REGISTRY_USER` + `REGISTRY_PASSWORD_FILE` |
| `registryCertDirs` | `IMAGE_TRUST_REGISTRY_CERT_DIRS` |
| `registryMirrors` | `IMAGE_TRUST_REGISTRY_MIRRORS` |

Prefer **password files** and **docker config** so credentials are not passed on the cosign command line.

## Multi-registry example (`IMAGE_TRUST_REGISTRY_AUTHS`)

```json
[
  {"host": "https://index.docker.io/v1/", "username": "user", "password": "token"},
  {"host": "https://ghcr.io", "username": "user", "password": "token"},
  {"host": "https://registry.example.com/v1/", "username": "robot", "password": "secret"}
]
```

## Registry mirrors

Set `IMAGE_TRUST_REGISTRY_MIRRORS` or `IMAGE_TRUST_REGISTRY_MIRRORS_FILE` (comma-separated `mirror=upstream` pairs):

```yaml
env:
  - name: IMAGE_TRUST_REGISTRY_MIRRORS
    value: "mirror.corp.io=registry.io,internal.cache=ghcr.io"
```

## Private Sigstore

Use `sigstore.env` for individual env vars, `sigstore.envFileSecret` for a mounted `KEY=VALUE` file, and `sigstore.trustBundleSecret` for `SIGSTORE_ROOT_FILE`:

```yaml
image-trust:
  sigstore:
    env:
      FULCIO_URL: https://fulcio.internal.example
      REKOR_URL: https://rekor.internal.example
    trustBundleSecret: sigstore-trust
    trustBundleSecretKey: root.pem
```

## KMS public keys (`cosign-key`)

```yaml
env:
  - name: IMAGE_TRUST_PUBLIC_KEY_REFS
    value: "gcpkms://projects/PROJECT/locations/LOC/keyRings/RING/cryptoKeys/KEY/versions/1"
  - name: GOOGLE_APPLICATION_CREDENTIALS
    value: /var/run/gcp/sa.json
```

For AWS EKS use `AWS_ROLE_ARN` + `AWS_WEB_IDENTITY_TOKEN_FILE`; for Azure use federated token env vars documented in the plugin README.

## Trusted public keys (`cosign-key`)

Clients supply **public** keys only (never private `cosign.key`). The plugin does not ship vendor keys in the container image — you configure trust at deploy time.

| Approach | Plugin env | Typical source |
|----------|------------|----------------|
| Directory of keys | `IMAGE_TRUST_PUBLIC_KEY_DIR` | Secret or ConfigMap mounted read-only; all `*.pub` / `*.pem` files in the directory are trusted |
| Explicit paths | `IMAGE_TRUST_PUBLIC_KEY_PATHS` | Comma-separated paths inside mounted volumes |
| Remote refs | `IMAGE_TRUST_PUBLIC_KEY_REFS` | KMS URIs or HTTPS URLs (cosign fetches at verify time; pod needs outbound access) |

### Mount keys from a Kubernetes Secret (recommended)

Create a Secret with one or more public key files (filenames become `signer.keyRef` in reports):

```bash
curl -fsSL -o fairwinds-cosign-p256.pub https://artifacts.fairwinds.com/cosign-p256.pub
kubectl -n insights create secret generic image-trust-public-keys \
  --from-file=fairwinds-cosign-p256.pub \
  --from-file=vendor-release.pub=./vendor-release.pub
```

Alternatively, skip the Secret and set `IMAGE_TRUST_PUBLIC_KEY_REFS=https://artifacts.fairwinds.com/cosign-p256.pub` if the pod can reach the URL.

Wire the Insights Agent chart (or your fork) so the image-trust container mounts the Secret and points at the directory:

```yaml
image-trust:
  enabled: true
  modes:
    - cosign-keyless
    - cosign-key
  ignoreTlog: true   # typical for static keys not in Rekor
  publicKeys:
    secretName: image-trust-public-keys
```

The chart mounts `publicKeys.secretName` at `/etc/image-trust/keys` and sets `IMAGE_TRUST_PUBLIC_KEY_DIR`.

Equivalent raw env on the plugin pod:

```yaml
env:
  - name: IMAGE_TRUST_MODES
    value: "cosign-keyless,cosign-key"
  - name: IMAGE_TRUST_PUBLIC_KEY_DIR
    value: /etc/image-trust/keys
  - name: IMAGE_TRUST_IGNORE_TLOG
    value: "true"
volumeMounts:
  - name: image-trust-public-keys
    mountPath: /etc/image-trust/keys
    readOnly: true
volumes:
  - name: image-trust-public-keys
    secret:
      secretName: image-trust-public-keys
```

Use `IMAGE_TRUST_PUBLIC_KEY_PATHS` when you mount keys outside a single directory or need an explicit allowlist:

```yaml
env:
  - name: IMAGE_TRUST_PUBLIC_KEY_PATHS
    value: /etc/image-trust/keys/fairwinds-cosign-p256.pub,/etc/image-trust/keys/vendor-release.pub
```

### Fairwinds OSS images

Fairwinds OSS release images (for example `us-docker.pkg.dev/fairwinds-ops/oss/polaris:v10.2.0+`) are signed with [cosign-p256.pub](https://artifacts.fairwinds.com/cosign-p256.pub). Mount that key from a Secret as above, or reference it directly (no volume) if the pod can reach the URL:

```yaml
env:
  - name: IMAGE_TRUST_PUBLIC_KEY_REFS
    value: "https://artifacts.fairwinds.com/cosign-p256.pub"
```

See [Polaris releases](https://github.com/FairwindsOps/polaris/releases) for the upstream verify commands.

`plugins/image-trust/testdata/keys/` and the smoke test mount are **local fixtures only** — not used in production images.

## Attestations

Enable with `attestations.enabled` and predicate types. Matching attestation modes are appended for each signature mode (`cosign-keyless` → `cosign-attestation-keyless`, `cosign-key` → `cosign-attestation-key`):

```yaml
image-trust:
  enabled: true
  modes:
    - cosign-keyless
  attestations:
    enabled: true
    types:
      - slsaprovenance1
      - spdxjson
```

Setting `attestations.types` alone (without `enabled`) also activates attestations. The plugin sets `attestationType` on verified images. Multiple predicate types are OR (any one match passes).

`modePolicy: all` requires **every** mode in `IMAGE_TRUST_MODES` to return `verified` — including `cosign-keyless` and `cosign-key` when both are enabled, and attestation modes when those are appended. With the recommended defaults (`modes: [cosign-keyless, cosign-key], modePolicy: any`), an image passes if **either** signature mode verifies. Use `all` only when you intentionally require multiple modes (for example keyless signature **and** a matching attestation).

## Recommended defaults

```yaml
image-trust:
  enabled: true
  resolveDigests: true
  verifyRetries: 3
  verifyRetryBackoffSeconds: 2
  verifyRetryJitter: true
  useImagePullSecrets: true
  modes:
    - cosign-keyless
    - cosign-key
  modePolicy: any
  publicKeys:
    refs:
      - https://artifacts.fairwinds.com/cosign-p256.pub
  ignoreTlog: true
```
