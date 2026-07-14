# This Dockerfile is used with goreleaser, see also .goreleaser.yml.envsubst.
# IG_VERSION comes from go.mod.
ARG IG_VERSION
FROM ghcr.io/inspektor-gadget/inspektor-gadget:${IG_VERSION}
COPY network-flow /usr/local/bin/network-flow
COPY entrypoint /usr/local/bin/entrypoint
ENTRYPOINT ["/usr/local/bin/entrypoint"]
