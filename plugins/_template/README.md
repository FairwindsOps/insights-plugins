# Template 

This is a template plugin. When adding a new plugin you can start by copying this directory and changing the values in the included files.

This ReadMe section should include a description of what your plugin does.

Both a goreleaser configuration and a Dockerfile are required (the included sample is for a Go-based plugin). Images still publish to Quay (`quay.io/fairwinds/<name>` — create the Quay repository if needed). The same short name is also pushed to Google Artifact Registry at `us-docker.pkg.dev/fairwinds-ops/oss/<name>` (shared `oss` repo; no new GAR repository required). GAR tags are immutable: only commit SHA and semver are pushed there; floating tags (`latest`, major, major.minor, feature) stay Quay-only.

## Running locally

If there are any special instructions needed for running the plugin locally then you can add those in this section.

## Documentation

If there is any external documentation then you can link to that here.
