# Test public keys

Place Cosign **public** keys (`.pub` / PEM) here for local experiments with `cosign-key` mode:

```bash
export IMAGE_TRUST_MODES=cosign-keyless,cosign-key
export IMAGE_TRUST_PUBLIC_KEY_DIR="$(pwd)/testdata/keys"
```

Never commit private `cosign.key` files.
