# Image Trust Plugin Plan

Status: Proposal

## Summary

Add a new plugin named `image-trust` to report whether container images running in a cluster are signed and trusted.

This plugin should be separate from `trivy` because the problem it solves is different:

- `trivy` answers: "What vulnerabilities are in this image?"
- `image-trust` answers: "Is this image signed by a trusted signer?"

The plugin should enumerate images currently used by workloads, verify each image by digest against a trust policy, and upload a dedicated Insights report containing actionable findings for unsigned or untrusted images.

## Problem

Today we scan images for vulnerabilities with `trivy`, but we do not report whether the image itself is signed.

That leaves a supply-chain gap:

- an image may be low-risk from a CVE perspective but still be unsigned
- an image may be signed, but not by a signer we trust
- registry or verification failures may hide real trust issues if we do not classify them clearly

We need a report similar to other Insights reports that tells users which running images are not compliant with image-signing expectations.

## Decision

Implement `image-trust` as a new plugin and report type.

Why this is the best fit:

- it matches the current repo model where report types map to distinct plugins
- it keeps trust verification separate from vulnerability scanning
- it avoids overloading the Trivy report format with unrelated concerns
- it allows separate rollout, configuration, and failure handling

## Why Not Extend Existing Plugins

### Not `trivy`

`trivy` already has the image discovery behavior we want, but its report format and purpose are vulnerability-focused. Folding trust verification into the same plugin would create a mixed report with two different meanings and two different remediation paths.

`trivy` is still the best place to reuse ideas and possibly code from, especially `plugins/trivy/pkg/image/getimages.go`.

### Not `workloads`

`workloads` is an inventory and metadata collector. Signature verification is slower, depends on registry and trust configuration, and is not just descriptive inventory. Keeping that behavior out of `workloads` keeps the plugin lightweight and avoids making cluster inventory depend on external verification steps.

## Goals

- discover all running container images in the cluster
- deduplicate images while preserving workload ownership metadata
- verify images by immutable digest
- classify each image into a trust status
- report non-compliant images in a format Insights can ingest and present
- support private registries and scoped rollout
- support on-demand execution like other report types

## Non-Goals

- replacing admission-control enforcement
- blocking workloads directly from this plugin
- replacing `trivy` or merging with vulnerability reporting
- implementing every signing technology in v1
- automatically deciding policy exceptions for third-party images

## Recommended V1 Scope

V1 should support:

- Cosign verification
- digest-based verification
- keyless verification using trusted issuer and subject matching
- key-based verification using configured public keys
- allowlists or exclusions for approved image patterns
- explicit classification of verification failures versus unsigned images

V1 should not require:

- Notation support
- attestation policy evaluation beyond basic signature trust
- historical drift analysis beyond the current report payload

## Trust Model

The plugin should not only answer "is this image signed?".

It should answer "does this image satisfy our trust policy?" because these are different cases:

1. `verified`: image has a valid signature from an allowed signer
2. `unsigned`: no matching signature is present
3. `signed_untrusted`: a signature exists, but the signer is not trusted by policy
4. `verification_error`: registry, auth, network, or tool failure prevented verification
5. `unknown`: image reference could not be resolved confidently enough to verify

This distinction matters because remediation is different for each case.

## High-Level Flow

1. Read plugin configuration from environment variables.
2. Enumerate all running images and their owners from the cluster.
3. Normalize images to stable verification targets, preferring digest references.
4. Deduplicate images so the same digest is verified only once.
5. Verify the image against the configured trust policy.
6. Attach verification results back to all owning workloads.
7. Build an Insights report.
8. Write `/output/image-trust.json`.
9. Upload the report as a new `image-trust` datatype.

## Plugin Layout

Recommended layout:

```text
plugins/image-trust/
  README.md
  CHANGELOG.md
  Dockerfile
  .goreleaser.yml.envsubst
  go.mod
  report.sh
  cmd/
    main.go
  pkg/
    config/
      config.go
      config_test.go
    image/
      getimages.go
      getimages_test.go
    models/
      report.go
    verify/
      cosign.go
      cosign_test.go
      classify.go
```

## Reuse Strategy

For V1, copy and adapt the image enumeration logic from `plugins/trivy/pkg/image/getimages.go`.

Why copy first instead of importing from `trivy`:

- it avoids a direct plugin-to-plugin dependency
- it keeps `image-trust` independently testable
- it reduces coupling between vulnerability scanning and trust verification

If the logic stays stable and is needed by more plugins later, it can be extracted into a shared package in a follow-up change.

## Verification Implementation

For the MVP, prefer using the `cosign` CLI from the plugin container rather than wiring the full Sigstore Go libraries immediately.

Reasons:

- this repo already uses external CLI tools in plugins such as `trivy` and `skopeo`
- the CLI path will be faster to implement and easier to debug in containerized execution
- it allows us to establish the report model and UX before optimizing internals

The plugin should verify by digest, not tag.

If the workload only references a tag, the plugin should use the resolved digest from Kubernetes status where available. If no trustworthy digest can be established, the result should be `unknown` or `verification_error`, not `verified`.

## Configuration

Suggested environment variables for V1:

- `NAMESPACE_ALLOWLIST`
- `NAMESPACE_BLOCKLIST`
- `IMAGE_TRUST_MODE`
- `IMAGE_TRUST_TRUSTED_ISSUERS`
- `IMAGE_TRUST_TRUSTED_SUBJECTS`
- `IMAGE_TRUST_TRUSTED_SUBJECT_REGEXPS`
- `IMAGE_TRUST_PUBLIC_KEYS`
- `IMAGE_TRUST_PUBLIC_KEY_FILE`
- `IMAGE_TRUST_EXCLUDED_IMAGE_PATTERNS`
- `IMAGE_TRUST_FAIL_OPEN`
- `REGISTRY_USER`
- `REGISTRY_PASSWORD`
- `REGISTRY_PASSWORD_FILE`
- `REGISTRY_CERT_DIR`

Configuration principles:

- support both keyless and key-based verification
- allow teams to scope rollout by namespace
- allow exclusions for known third-party images
- keep verification errors separate from policy failures

## Report Shape

The exact Insights schema can be adjusted later, but the plugin output should contain both image-level and owner-level context.

Suggested top-level shape:

```json
{
  "images": [
    {
      "name": "ghcr.io/example/app:1.2.3",
      "id": "ghcr.io/example/app@sha256:abc123",
      "status": "unsigned",
      "reason": "no matching signatures found",
      "owners": [
        {
          "namespace": "production",
          "kind": "Deployment",
          "name": "api",
          "container": "api"
        }
      ],
      "lastCheckedAt": "2026-05-26T00:00:00Z"
    }
  ]
}
```

If Insights requires an action-item-oriented structure instead, the plugin should map each non-compliant image owner to a finding with:

- resource namespace
- resource kind
- resource name
- title
- description
- remediation
- severity
- category

Recommended title examples:

- `Container image is unsigned`
- `Container image is signed by an untrusted signer`
- `Container image trust could not be verified`

## Remediation Guidance

The report should produce remediation text that matches the trust state:

- `unsigned`: sign the image in CI and redeploy using a verified digest
- `signed_untrusted`: update signing configuration or use an approved signing identity
- `verification_error`: fix registry credentials, network access, or trust configuration and rerun
- `unknown`: ensure the workload resolves to a digest or that image metadata is available

## On-Demand Job Support

Add a new report type to `plugins/on-demand-job-runner/pkg/ondemandjobs/processor.go`:

- `image-trust`

This should map to a new CronJob name:

- `image-trust`

This keeps the plugin aligned with the existing on-demand report architecture.

## Uploader Integration

Follow the same model used by other plugins:

- the plugin writes a final file to `/output/image-trust.json`
- the uploader posts it to `/data/image-trust`

If the backend needs a different datatype name, that can be adjusted later, but the plugin should be built around a dedicated report type from the start.

## Phased Implementation Plan

### Phase 1: Scaffold

Create the new plugin directory from `plugins/_template` and add:

- `go.mod`
- `cmd/main.go`
- `README.md`
- `report.sh`
- `Dockerfile`
- release metadata

Exit criteria:

- plugin builds
- plugin runs
- plugin writes a valid empty or stub report

### Phase 2: Image Discovery

Add image enumeration modeled after `trivy`.

Exit criteria:

- plugin can list all images in a cluster
- owner metadata is preserved
- namespace allowlist and blocklist work
- duplicate images are verified once

### Phase 3: Verification MVP

Integrate Cosign-based verification.

Exit criteria:

- plugin can classify images as `verified`, `unsigned`, or `verification_error`
- digest verification works for the common case

### Phase 4: Trust Policy Expansion

Add signer policy support.

Exit criteria:

- keyless identity matching works
- key-based verification works
- `signed_untrusted` is reported separately from `unsigned`

### Phase 5: Report Quality

Shape findings for Insights and add remediation text.

Exit criteria:

- report is useful in product without manual interpretation
- findings include owner references and clear remediation

### Phase 6: Hardening

Improve reliability and operator usability.

Exit criteria:

- private registry auth is supported
- exclusions work
- retry and timeout behavior is predictable
- logs are clear enough for support and debugging

## Testing Plan

### Unit Tests

- config parsing
- image discovery filters and deduplication
- trust status classification
- error classification
- report generation

### Fixture Tests

Use known signed and unsigned image examples to validate:

- unsigned image
- signed image with trusted identity
- signed image with untrusted identity
- registry authentication failure
- digest missing or unresolved case

### End-to-End Tests

Smoke test the plugin in a cluster with:

- at least one signed first-party image
- at least one unsigned image
- optional private registry case

## Rollout Plan

Recommended rollout sequence:

1. release plugin with audit-style reporting only
2. validate result quality in internal or test clusters
3. refine exclusion and policy configuration
4. enable on-demand runs in product
5. promote as a supported report type

This plugin should complement admission-control-based enforcement, not replace it.

## Risks

- false positives if digest resolution is incomplete
- operator confusion if unsigned and verification failures are mixed together
- registry auth complexity for private images
- support burden if trust configuration is too flexible in v1

## Open Questions

- Should the backend store all image states or only non-compliant findings?
- What should the final datatype name be in the Insights API?
- Should the initial severity be fixed or status-dependent?
- Should `verification_error` generate a finding by default or only a warning?
- Do we want Notation support in a later phase, or is Cosign enough for the first release?

## Recommended Next Step

Implement Phase 1 and Phase 2 first, with a stub verifier if necessary, so that image discovery and report shape can be reviewed before trust-policy details are finalized.
