# Template 

This is a template plugin. When adding a new plugin you can start by copying this directory and changing the values in the included files.

This ReadMe section should include a description of what your plugin does.

Both a goreleaser configuration and a Dockerfile are required (the included sample is for a Go-based plugin). Images dual-push to Quay (`quay.io/fairwinds/<name>` — create the Quay repository if needed) and Google Artifact Registry (`us-docker.pkg.dev/fairwinds-ops/oss/<name>`, shared `oss` repo). CI scan / Insights image lists prefer GAR and pin semver. GAR tags are immutable (commit SHA + semver only); floating tags (`latest`, major, major.minor, feature) stay Quay-only.

## Running locally

If there are any special instructions needed for running the plugin locally then you can add those in this section.

## Documentation

If there is any external documentation then you can link to that here.
