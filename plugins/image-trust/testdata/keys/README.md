# Test public keys

Place Cosign **public** keys (`.pub` / PEM) here for local experiments with `cosign-key` mode:

```bash
export IMAGE_TRUST_MODES=cosign-keyless,cosign-key
export IMAGE_TRUST_PUBLIC_KEY_DIR="$(pwd)/testdata/keys"
```

Remote KMS public keys (no local file):

```bash
export IMAGE_TRUST_PUBLIC_KEY_REFS='gcpkms://projects/.../cryptoKeyVersions/1'
```

Never commit private `cosign.key` files.
