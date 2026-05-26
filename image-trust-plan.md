# Image Trust Plugin Plan

Status: Proposal

## Summary

Add a new plugin named `image-trust` to report whether container images running in a cluster are signed and trusted.

This plugin should be separate from `trivy` because the problem it solves is different:

- `trivy` answers: "What vulnerabilities are in this image?"
- `image-trust` answers: "Is this image signed by a trusted signer?"

The plugin should enumerate images currently used by workloads, verify each image by digest against a trust policy, and upload a dedicated Insights report containing a trust result for every image plus actionable findings for non-compliant images.

## Problem

Today we scan images for vulnerabilities with `trivy`, but we do not report whether the image itself is signed.

That leaves a supply-chain gap:

- an image may be low-risk from a CVE perspective but still be unsigned
- an image may be signed, but not by a signer we trust
- registry or verification failures may hide real trust issues if we do not classify them clearly

We need a report similar to other Insights reports that tells users the trust state of every running image and clearly identifies which images are not compliant with image-signing expectations.

## Decision

Implement `image-trust` as a new plugin and report type.

Why this is the best fit:

- it matches the current repo model where report types map to distinct plugins
- it keeps trust verification separate from vulnerability scanning
- it avoids overloading the Trivy report format with unrelated concerns
- it allows separate rollout, configuration, and failure handling

Additional product decisions:

- send all image results to Insights, not just failing ones
- upload through the standard report endpoint used by other plugins
- support multiple popular verification methods behind a common verifier interface
- add allowlists for approved images and trusted signers

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
- report every image in a format Insights can ingest and present
- expose specific per-image trust results and reasons
- derive findings from those results for non-compliant images
- support private registries and scoped rollout
- support allowlists for approved image patterns, registries, and signers
- support multiple popular verification methods
- support on-demand execution like other report types

## Non-Goals

- replacing admission-control enforcement
- blocking workloads directly from this plugin
- replacing `trivy` or merging with vulnerability reporting
- automatically deciding policy exceptions for third-party images

## Recommended V1 Scope

V1 should support:

- Cosign keyless verification
- digest-based verification
- keyless verification using trusted issuer and subject matching
- key-based verification using configured public keys
- certificate-based verification where verifier metadata can be mapped to trusted identities
- KMS-backed signing flows when they are verified through Cosign-compatible trust material
- Notation / Notary Project verification
- allowlists or exclusions for approved image patterns, registries, and signers
- explicit classification of verification failures versus unsigned images
- reporting all images, even when compliant

V1 should not require:

- attestation policy evaluation beyond basic signature trust
- historical drift analysis beyond the current report payload
- Docker Content Trust / Notary v1 support unless a strong product need emerges

## Verification Modes

The plugin should be designed around a verifier interface so we can support multiple trust mechanisms without rewriting report generation.

Initial verification modes:

1. `cosign-keyless`
   - trusted by issuer plus subject or subject regexp
2. `cosign-key`
   - trusted by public key material
3. `cosign-certificate`
   - trusted by certificate identity constraints where applicable
4. `cosign-kms`
   - treated as a Cosign-backed verification mode, with trust rooted in configured keys or certificate identity
5. `notation`
   - Notary Project / Notation verification for OCI artifacts

The plugin should normalize verifier output into one common trust result model.

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
7. Build an Insights report containing all image results.
8. Write `/output/image-trust.json`.
9. Upload the report as a new `image-trust` datatype through the same standard `/data/$datatype` endpoint other plugins use.

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
    discovery/
      images.go
      images_test.go
    normalize/
      references.go
      references_test.go
    models/
      image.go
      result.go
      report.go
    policy/
      allowlist.go
      allowlist_test.go
      trustpolicy.go
      trustpolicy_test.go
    report/
      builder.go
      builder_test.go
    verify/
      verifier.go
      cosign.go
      cosign_test.go
      notation.go
      notation_test.go
      classify.go
      classify_test.go
    output/
      write.go
      write_test.go
```

## Clean Code Principles

The implementation should optimize for clarity and changeability over minimizing file count.

Guidelines:

- keep `cmd/main.go` thin and focused on wiring
- isolate external side effects behind small interfaces
- keep trust status classification deterministic and centralized
- separate image discovery, policy evaluation, verification, and report building
- prefer small structs with explicit fields over generic `map[string]any`
- return typed errors or structured result codes where practical
- avoid hidden behavior in package globals
- keep allowlist behavior explicit and testable
- design each verifier as an adapter behind a shared interface

## Architecture Overview

Recommended internal flow:

1. `config` loads environment variables into a validated runtime config
2. `discovery` fetches cluster images and owner references
3. `normalize` produces stable verification targets and deduplicates them
4. `verify` runs one or more verifier adapters against each image
5. `policy` evaluates trust rules and allowlists against verifier output
6. `report` converts image results into the final Insights payload
7. `output` writes the report file for the uploader

This gives the plugin one clear orchestration path while keeping the logic independently testable.

## Dependency Direction

Package dependencies should flow inward toward simple models:

- `cmd` may depend on all internal packages
- `config` should depend only on standard library helpers and `models` if needed
- `discovery`, `normalize`, `verify`, `policy`, `report`, and `output` should depend on `models`
- `report` should not call verifier code directly
- `policy` should not know about CLI commands or file system details
- verifier implementations should not build final Insights reports

This keeps business rules separate from transport and tooling concerns.

## Core Interfaces

Suggested interfaces:

```go
type ImageDiscoverer interface {
    ListImages(ctx context.Context) ([]models.DiscoveredImage, error)
}

type Verifier interface {
    Name() string
    Verify(ctx context.Context, image models.VerificationTarget) (models.VerificationObservation, error)
}

type AllowlistMatcher interface {
    Match(image models.VerificationTarget, observation models.VerificationObservation) *models.AllowlistMatch
}

type ReportBuilder interface {
    Build(results []models.ImageTrustResult) models.Report
}
```

Notes:

- `Verifier` should return raw observation data, not final policy status
- final status should be assigned in one place after allowlist and trust-policy evaluation
- this prevents every verifier from reimplementing policy decisions differently

## Data Model Plan

Keep a small number of explicit model types:

- `DiscoveredImage`
  - image reference as seen in cluster
  - resolved digest if available
  - owners
- `VerificationTarget`
  - normalized immutable target used for verification
- `VerificationObservation`
  - raw verifier output
  - verifier name
  - signer details
  - tool-specific reason
- `AllowlistMatch`
  - whether matched
  - rule type
  - rule value
- `ImageTrustResult`
  - final status
  - reason
  - allowlisted flag
  - owners
- `Report`
  - full image results plus summary and derived findings

This split helps keep verification facts separate from policy decisions.

## Package Responsibilities

### `pkg/config`

Responsible for:

- environment parsing
- default values
- validation
- compiling regex patterns once

Should not be responsible for:

- verification logic
- report generation

### `pkg/discovery`

Responsible for:

- Kubernetes image discovery
- namespace filtering
- owner aggregation

Should not be responsible for:

- trust policy
- allowlist evaluation
- external verifier invocation

### `pkg/normalize`

Responsible for:

- choosing the best immutable verification target
- canonicalizing image IDs
- deduplicating equivalent images

This is important because digest-resolution mistakes are one of the biggest correctness risks.

### `pkg/verify`

Responsible for:

- wrapping `cosign` and `notation`
- parsing command output
- returning normalized observations

Should not be responsible for:

- deciding if an allowlist should suppress a finding
- deciding final severity

### `pkg/policy`

Responsible for:

- trust policy evaluation
- allowlist evaluation
- merging verifier observations into final result states

This should be the only place that decides whether a result is:

- `verified`
- `unsigned`
- `signed_untrusted`
- `verification_error`
- `unknown`

### `pkg/report`

Responsible for:

- converting final results into the report payload
- generating summary counts
- deriving findings for non-compliant results

### `pkg/output`

Responsible for:

- writing the final JSON payload
- ensuring stable file output behavior

## Coding Roadmap

Implementation should happen in small reviewable slices.

### Slice 1: Skeleton and Models

Deliver:

- plugin scaffolding
- core model types
- config loading
- empty report output

Definition of done:

- binary runs
- writes valid JSON
- unit tests cover config and report writing

### Slice 2: Discovery and Normalization

Deliver:

- image discovery copied and adapted from Trivy
- normalized verification targets
- deterministic deduplication

Definition of done:

- test fixtures cover duplicate owners, namespace filters, missing digests

### Slice 3: Verifier Interface and Cosign Adapter

Deliver:

- `Verifier` interface
- `cosign-keyless` implementation
- raw observation model

Definition of done:

- command execution is isolated behind testable helpers
- parser tests cover success, unsigned, and command failure paths

### Slice 4: Policy and Allowlists

Deliver:

- trust policy evaluator
- image, registry, and signer allowlists
- final result classifier

Definition of done:

- unit tests cover precedence and suppression rules

### Slice 5: Report Builder

Deliver:

- full image report
- summary counts
- derived findings

Definition of done:

- report builder tests use table-driven inputs for all statuses

### Slice 6: Additional Verifiers

Deliver:

- `cosign-key`
- `notation`
- optional certificate-oriented trust mapping

Definition of done:

- new verifiers plug in without changing report or policy packages

### Slice 7: Operational Hardening

Deliver:

- retry and timeout policy
- clearer structured logs
- private registry support refinements

Definition of done:

- end-to-end smoke tests pass for public and private registry scenarios

## Reuse Strategy

For V1, copy and adapt the image enumeration logic from `plugins/trivy/pkg/image/getimages.go`.

Why copy first instead of importing from `trivy`:

- it avoids a direct plugin-to-plugin dependency
- it keeps `image-trust` independently testable
- it reduces coupling between vulnerability scanning and trust verification

If the logic stays stable and is needed by more plugins later, it can be extracted into a shared package in a follow-up change.

## Verification Implementation

For the first implementation, prefer using external CLIs from the plugin container rather than wiring full signing libraries immediately.

Reasons:

- this repo already uses external CLI tools in plugins such as `trivy` and `skopeo`
- the CLI path will be faster to implement and easier to debug in containerized execution
- it allows us to establish the report model and UX before optimizing internals

Recommended tools:

- `cosign` for Sigstore verification modes
- `notation` for Notary Project verification

The plugin should verify by digest, not tag.

If the workload only references a tag, the plugin should use the resolved digest from Kubernetes status where available. If no trustworthy digest can be established, the result should be `unknown` or `verification_error`, not `verified`.

Each verifier should return:

- normalized image ID
- verification mode used
- trust status
- human-readable reason
- signer details when available

## Configuration

Suggested environment variables for V1:

- `NAMESPACE_ALLOWLIST`
- `NAMESPACE_BLOCKLIST`
- `IMAGE_TRUST_MODES`
- `IMAGE_TRUST_TRUSTED_ISSUERS`
- `IMAGE_TRUST_TRUSTED_SUBJECTS`
- `IMAGE_TRUST_TRUSTED_SUBJECT_REGEXPS`
- `IMAGE_TRUST_PUBLIC_KEYS`
- `IMAGE_TRUST_PUBLIC_KEY_FILE`
- `IMAGE_TRUST_TRUSTED_SIGNER_ALLOWLIST`
- `IMAGE_TRUST_IMAGE_ALLOWLIST`
- `IMAGE_TRUST_REGISTRY_ALLOWLIST`
- `IMAGE_TRUST_EXCLUDED_IMAGE_PATTERNS`
- `IMAGE_TRUST_FAIL_OPEN`
- `REGISTRY_USER`
- `REGISTRY_PASSWORD`
- `REGISTRY_PASSWORD_FILE`
- `REGISTRY_CERT_DIR`

Configuration principles:

- support multiple verification methods
- support both keyless and key-based verification
- allow teams to scope rollout by namespace
- allow exclusions for known third-party images
- allow explicit allowlists for approved images, registries, and signers
- keep verification errors separate from policy failures

## Report Shape

The plugin output should contain both image-level and owner-level context, and it should include all images, not only failing ones.

Suggested top-level shape:

```json
{
  "images": [
    {
      "name": "ghcr.io/example/app:1.2.3",
      "id": "ghcr.io/example/app@sha256:abc123",
      "status": "unsigned",
      "verificationMode": "cosign-keyless",
      "reason": "no matching signatures found",
      "allowlisted": false,
      "signer": {
        "issuer": "",
        "subject": "",
        "keyRef": ""
      },
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
  ],
  "summary": {
    "totalImages": 100,
    "verified": 72,
    "unsigned": 11,
    "signedUntrusted": 7,
    "verificationError": 8,
    "unknown": 2,
    "allowlisted": 14
  }
}
```

The report should be sent to the same standard report endpoint other plugins use:

- `/v0/organizations/$organization/clusters/$cluster/data/image-trust`

In addition to the full image list, the plugin should derive per-owner findings for non-compliant images with:

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
- `Container image trust is allowlisted`

## Remediation Guidance

The report should produce remediation text that matches the trust state:

- `unsigned`: sign the image in CI and redeploy using a verified digest
- `signed_untrusted`: update signing configuration or use an approved signing identity
- `verification_error`: fix registry credentials, network access, or trust configuration and rerun
- `unknown`: ensure the workload resolves to a digest or that image metadata is available

Allowlisted images should still be reported in the full image list, but findings for them should be suppressed or specially marked depending on product behavior.

## Allowlist Behavior

Allowlists should be explicit policy inputs, not implicit passes.

Recommended allowlist types:

- image pattern allowlist
- registry allowlist
- signer allowlist
- namespace-scoped allowlist if needed later

Recommended behavior:

- allowlisted images still appear in the report
- allowlisted images carry `allowlisted: true`
- allowlisted images are excluded from failing findings by default
- allowlist matches should record why the image was exempted

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
- this should use the generic uploader path rather than a one-off endpoint

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

Integrate the verifier interface and implement at least one end-to-end mode.

Exit criteria:

- plugin can classify images as `verified`, `unsigned`, or `verification_error`
- digest verification works for the common case
- report includes all images, not just failures

### Phase 4: Trust Policy Expansion

Add multiple verification modes and allowlist support.

Exit criteria:

- keyless identity matching works
- key-based verification works
- notation verification works
- `signed_untrusted` is reported separately from `unsigned`
- allowlisted images are present in the report and suppressed from findings by policy

### Phase 5: Report Quality

Shape the full report plus derived findings for Insights and add remediation text.

Exit criteria:

- report is useful in product without manual interpretation
- report includes all image states and aggregate counts
- findings include owner references and clear remediation

### Phase 6: Hardening

Improve reliability and operator usability.

Exit criteria:

- private registry auth is supported
- exclusions work
- retry and timeout behavior is predictable
- logs are clear enough for support and debugging

## Testing Plan

Testing should favor fast unit tests first, then a smaller number of integration and end-to-end checks.

Principles:

- use table-driven tests for policy and status mapping
- use fixture-based tests for command output parsing
- keep external CLI execution behind injectable helpers or interfaces
- avoid requiring a live cluster for most tests
- keep one or two smoke tests for real verifier behavior

### Unit Tests

- config parsing
- image discovery filters and deduplication
- normalization and digest selection
- trust status classification
- allowlist precedence
- error classification
- report generation

### Fixture Tests

Use known signed and unsigned image examples to validate:

- unsigned image
- signed image with trusted identity
- signed image with untrusted identity
- notation-verified image
- allowlisted unsigned image
- registry authentication failure
- digest missing or unresolved case

Fixture sources should include:

- captured `cosign verify` output
- captured `notation verify` output
- normalized fake image discovery inputs
- sample allowlist configurations

### End-to-End Tests

Smoke test the plugin in a cluster with:

- at least one signed first-party image
- at least one unsigned image
- at least one allowlisted image
- optional private registry case

## Testability Requirements

The code should be written so these components can be tested in isolation:

- config validation without Kubernetes access
- discovery without verifier binaries
- verifier parsing without real clusters
- policy evaluation without shelling out
- report generation without any external dependencies

If a package cannot be tested without a live registry or cluster, the package boundary should be revisited.

## Rollout Plan

Recommended rollout sequence:

1. release plugin with audit-style reporting only
2. validate result quality in internal or test clusters
3. refine allowlist and policy configuration
4. enable on-demand runs in product
5. promote as a supported report type

This plugin should complement admission-control-based enforcement, not replace it.

## Risks

- false positives if digest resolution is incomplete
- operator confusion if unsigned and verification failures are mixed together
- registry auth complexity for private images
- support burden if trust configuration is too flexible in v1
- scope and maintenance cost increase if we support multiple verification systems from day one

## Open Questions

- Should the initial severity be fixed or status-dependent?
- Should `verification_error` generate a finding by default or only a warning?
- What is the exact precedence between allowlists and verification failures?
- Do we need Docker Content Trust support, or are Cosign and Notation sufficient for "popular" verification methods?

## Recommended Next Step

Implement Phase 1 and Phase 2 first, with a stub verifier if necessary, so that image discovery and report shape can be reviewed before trust-policy details are finalized.
