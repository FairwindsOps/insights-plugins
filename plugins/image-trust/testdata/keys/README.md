# Test public keys

**Local / smoke test fixtures only.** Production and chart deployments mount trusted public keys via Kubernetes Secrets, ConfigMaps, or remote refs configured by the client — see [CHART_INTEGRATION.md](../../CHART_INTEGRATION.md#trusted-public-keys-cosign-key).

## Fairwinds OSS release signing (smoke test)

[`fairwinds-cosign-p256.pub`](fairwinds-cosign-p256.pub) is the P-256 release signing key published at [artifacts.fairwinds.com/cosign-p256.pub](https://artifacts.fairwinds.com/cosign-p256.pub). Fairwinds OSS container images (for example [Polaris v10.2.0+](https://github.com/FairwindsOps/polaris/releases)) are signed with this key.

```bash
cosign verify us-docker.pkg.dev/fairwinds-ops/oss/polaris:v10.2.0 \
  --key testdata/keys/fairwinds-cosign-p256.pub \
  --insecure-ignore-tlog
```

The smoke test defaults to `IMAGE_TRUST_PUBLIC_KEY_REFS=https://artifacts.fairwinds.com/cosign-p256.pub`. This file is kept for local `cosign verify --key` and optional `IMAGE_TRUST_PUBLIC_KEY_DIR` overrides.

## Other keys

Add vendor `.pub` / PEM files here for additional keyed verification:

```bash
export IMAGE_TRUST_MODES=cosign-keyless,cosign-key
export IMAGE_TRUST_PUBLIC_KEY_DIR="$(pwd)/testdata/keys"
```

Remote KMS public keys (no local file):

```bash
export IMAGE_TRUST_PUBLIC_KEY_REFS='gcpkms://projects/.../cryptoKeyVersions/1'
```

Never commit private `cosign.key` files.
