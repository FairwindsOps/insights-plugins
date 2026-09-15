FROM alpine:3.24.1 AS downloader
ARG TARGETARCH
ENV COSIGN_VERSION=3.1.3
RUN apk --no-cache add ca-certificates curl \
    && if [ "${TARGETARCH}" = "amd64" ]; then \
         cosign_arch="amd64"; \
         cosign_sha256="4629c757b7618056f8ddd7e2625ae9fdd94c0372a65049520bc7d9df9efc7f71"; \
       elif [ "${TARGETARCH}" = "arm64" ]; then \
         cosign_arch="arm64"; \
         cosign_sha256="c5d324e091826b0d7a78eb16fef316450b4eb9aaec045611c08ba06f5e73220a"; \
       else \
         echo "unsupported TARGETARCH: ${TARGETARCH}" >&2; exit 1; \
       fi \
    && curl -fsSL "https://github.com/sigstore/cosign/releases/download/v${COSIGN_VERSION}/cosign-linux-${cosign_arch}" -o /usr/local/bin/cosign \
    && echo "${cosign_sha256}  /usr/local/bin/cosign" | sha256sum -c - \
    && chmod +x /usr/local/bin/cosign

FROM alpine:3.24.1
WORKDIR /usr/local/bin
RUN apk --no-cache add ca-certificates \
    && apk --no-cache add --upgrade libcrypto3 libssl3 zlib

RUN addgroup -S insights && adduser -u 1200 -S insights -G insights \
    && mkdir -p /output \
    && chown -R insights:insights /output

COPY image-trust .
COPY --from=downloader /usr/local/bin/cosign /usr/local/bin/cosign
COPY report.sh /opt/app/report.sh
RUN chmod +x /opt/app/report.sh

USER 1200
WORKDIR /opt/app

CMD ["./report.sh"]
