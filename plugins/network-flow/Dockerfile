ARG IG_VERSION=v0.52.0
FROM ghcr.io/inspektor-gadget/inspektor-gadget:${IG_VERSION}
COPY network-flow /usr/local/bin/network-flow
COPY entrypoint /usr/local/bin/entrypoint
ENTRYPOINT ["/usr/local/bin/entrypoint"]
