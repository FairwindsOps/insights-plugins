# Image Trust

Reports image trust for container images running in a cluster.

The plugin discovers images used by workloads (including init and ephemeral containers), verifies signatures with Cosign keyless, applies trust policy and allowlists, and uploads a report to Insights at `/data/image-trust`.

## Report output

- **images**: trust result for every discovered image (verified, unsigned, signed_untrusted, verification_error, unknown)
- **summary**: counts by status
- **ActionItems**: findings for non-compliant images (allowlisted images are listed but do not generate findings)

Output file: `/output/image-trust.json` (via `report.sh` and the uploader).

## Configuration

Namespace scope:

- `NAMESPACE_ALLOWLIST`
- `NAMESPACE_BLOCKLIST`

Verification:

- `IMAGE_TRUST_MODES` — default `cosign-keyless` (only supported mode today)
- `IMAGE_TRUST_TRUSTED_ISSUERS` — **required** unless subjects/regexps are set; comma-separated OIDC issuers
- `IMAGE_TRUST_TRUSTED_SUBJECTS` — exact certificate identities
- `IMAGE_TRUST_TRUSTED_SUBJECT_REGEXPS` — identity regexes (max 32 patterns, 512 characters each)
- `MAX_CONCURRENT_SCANS` — parallel cosign verifications (default `5`)
- `IMAGE_VERIFY_TIMEOUT_SECONDS` — per-image verify timeout (default `180`)

At least one of `IMAGE_TRUST_TRUSTED_ISSUERS`, `IMAGE_TRUST_TRUSTED_SUBJECTS`, or `IMAGE_TRUST_TRUSTED_SUBJECT_REGEXPS` must be configured.

Allowlists (glob patterns; findings suppressed when matched):

- `IMAGE_TRUST_IMAGE_ALLOWLIST`
- `IMAGE_TRUST_REGISTRY_ALLOWLIST`
- `IMAGE_TRUST_SIGNER_ALLOWLIST` — matches issuer, subject, or `issuer|subject`

Registry credentials (private images; same variables as the Trivy plugin):

- `REGISTRY_USER`, `REGISTRY_PASSWORD`, `REGISTRY_PASSWORD_FILE`, `REGISTRY_CERT_DIR`

Private registry verification requires outbound access to the registry and, for keyless signatures, to Sigstore services (Fulcio, Rekor, and TUF roots).

## Product integration (this repo)

| Component | Status |
|-----------|--------|
| Plugin binary + Docker image | `quay.io/fairwinds/image-trust` |
| Uploader datatype | `image-trust` → `/data/image-trust` |
| On-demand jobs | `image-trust` report type in `on-demand-job-runner` |
| JSON schema | `plugins/image-trust/results.schema` |

**Fairwinds Insights Agent chart** (separate [charts](https://github.com/FairwindsOps/charts) repo): add a CronJob/uploader pair for `image-trust` mirroring `trivy` — enable flag, image tag, env for trust policy and allowlists, schedule, and resources.

## Running locally

Build from the repository root so GoReleaser artifacts line up with other plugins:

```bash
docker build -t fw-image-trust -f plugins/image-trust/Dockerfile plugins/image-trust
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

Cosign is included in the published Dockerfile with release checksum verification.
