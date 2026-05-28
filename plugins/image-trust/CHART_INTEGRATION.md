# Insights Agent chart integration

The Fairwinds Insights Agent chart lives in [FairwindsOps/charts](https://github.com/FairwindsOps/charts). Wire these values when enabling image-trust.

## RBAC (required for pull secrets)

When `IMAGE_TRUST_USE_IMAGE_PULL_SECRETS=true`, bind the ClusterRole in [deploy/rbac.yaml](deploy/rbac.yaml) to the image-trust ServiceAccount. Example chart fragment:

```yaml
image-trust:
  enabled: true
  useImagePullSecrets: true
  rbac:
    create: true
```

Minimum rules: `secrets` and `namespaces` **list/get** cluster-wide (or scoped via Role + namespace list if you fork the chart).

## Registry credentials

| Chart value / env | Plugin env |
|-------------------|------------|
| `privateImages.registryAuths` | `IMAGE_TRUST_REGISTRY_AUTHS` (JSON array) |
| `privateImages.dockerConfigSecret` | `REGISTRY_DOCKER_CONFIG_PATH` |
| `privateImages.username` / `passwordSecret` | `REGISTRY_USER` + `REGISTRY_PASSWORD_FILE` |
| `privateImages.certDirs` | `IMAGE_TRUST_REGISTRY_CERT_DIRS` |

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

```yaml
env:
  - name: IMAGE_TRUST_REGISTRY_MIRRORS
    value: "mirror.corp.io=registry.io,internal.cache=ghcr.io"
```

## Private Sigstore

Mount trust roots and set env on the image-trust container (or use `IMAGE_TRUST_SIGSTORE_ENV_FILE`):

```yaml
env:
  - name: SIGSTORE_ROOT_FILE
    value: /etc/sigstore/root.pem
  - name: FULCIO_URL
    value: https://fulcio.internal.example
  - name: REKOR_URL
    value: https://rekor.internal.example
volumeMounts:
  - name: sigstore-trust
    mountPath: /etc/sigstore
    readOnly: true
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

## Attestations

Enable with `attestations.enabled` and predicate types. Matching attestation modes are appended for each signature mode (`cosign-keyless` → `cosign-attestation-keyless`, `cosign-key` → `cosign-attestation-key`):

```yaml
image-trust:
  modes:
    - cosign-keyless
  attestations:
    enabled: true
    types:
      - slsaprovenance1
      - spdxjson
```

Setting `attestations.types` alone (without `enabled`) also activates attestations. The plugin sets `attestationType` on verified images. Multiple predicate types are OR (any one match passes).

Use `modePolicy: all` when an image must pass **both** signature and attestation verification.

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
```
