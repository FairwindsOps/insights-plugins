# Image Trust

Reports image trust for container images running in a cluster.

The plugin discovers images used by workloads, verifies signatures with Cosign (keyless by default), applies trust policy and allowlists, and uploads a report to Insights at `/data/image-trust`.

## Report output

- **images**: trust result for every discovered image (verified, unsigned, signed_untrusted, verification_error, unknown)
- **summary**: counts by status
- **ActionItems**: findings for non-compliant images (allowlisted images are listed but do not generate findings)

Output file: `/output/image-trust.json` (via `report.sh` and the uploader).

## Configuration

Namespace scope (same pattern as Trivy):

- `NAMESPACE_ALLOWLIST`
- `NAMESPACE_BLOCKLIST`

Verification:

- `IMAGE_TRUST_MODES` — default `cosign-keyless`
- `IMAGE_TRUST_TRUSTED_ISSUERS` — comma-separated OIDC issuers
- `IMAGE_TRUST_TRUSTED_SUBJECTS` — exact certificate identities
- `IMAGE_TRUST_TRUSTED_SUBJECT_REGEXPS` — identity regexes (use `**` for multi-segment paths in allowlists)

Allowlists (glob patterns; findings suppressed when matched):

- `IMAGE_TRUST_IMAGE_ALLOWLIST`
- `IMAGE_TRUST_REGISTRY_ALLOWLIST`
- `IMAGE_TRUST_SIGNER_ALLOWLIST` — matches issuer, subject, or `issuer|subject`

Registry credentials (private images):

- `REGISTRY_USER`, `REGISTRY_PASSWORD`, `REGISTRY_PASSWORD_FILE`, `REGISTRY_CERT_DIR`

## Product integration (this repo)

| Component | Status |
|-----------|--------|
| Plugin binary + Docker image | `quay.io/fairwinds/image-trust` |
| Uploader datatype | `image-trust` → `/data/image-trust` |
| On-demand jobs | `image-trust` report type in `on-demand-job-runner` |
| JSON schema | `plugins/image-trust/results.schema` |

**Fairwinds Insights Agent chart** (separate [charts](https://github.com/FairwindsOps/charts) repo): add a CronJob/uploader pair for `image-trust` mirroring `trivy` — enable flag, image tag, env for trust policy and allowlists, schedule, and resources.

## Running locally

```bash
docker build -t fw-image-trust plugins/image-trust
docker run --network host \
  -e KUBECONFIG=/root/.kubeconfig \
  -v $HOME/.kube/config:/root/.kubeconfig \
  -v $(pwd)/output:/output \
  fw-image-trust
```

Ensure `cosign` is available in the image (included in the published Dockerfile).
