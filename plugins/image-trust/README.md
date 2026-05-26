# Image Trust

Reports image trust data for container images running in a cluster.

The initial implementation focuses on:

- discovering images used by workloads
- producing a structured `image-trust` report
- establishing clean package boundaries for later verification work

Future phases will add image signature verification, allowlists, and additional trust evidence.

## Running locally

This plugin writes its final report to `/output/image-trust.json` when run through `report.sh`.
