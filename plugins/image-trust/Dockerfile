FROM alpine:3.24.3 AS downloader
ARG TARGETARCH
ENV COSIGN_VERSION=3.1.4
RUN apk --no-cache add ca-certificates curl \
    && if [ "${TARGETARCH}" = "amd64" ]; then \
         cosign_arch="amd64"; \
         cosign_sha256="424e2c97d0b45fd7a6b608a87f090f35633f4c23ece5f092d48bba11aaf4bbfa"; \
       elif [ "${TARGETARCH}" = "arm64" ]; then \
         cosign_arch="arm64"; \
         cosign_sha256="ade83b7aac62e131cacfe96d958948b2150cc9efec46a570a2649b8e27dedad2"; \
       else \
         echo "unsupported TARGETARCH: ${TARGETARCH}" >&2; exit 1; \
       fi \
    && curl -fsSL "https://github.com/sigstore/cosign/releases/download/v${COSIGN_VERSION}/cosign-linux-${cosign_arch}" -o /usr/local/bin/cosign \
    && echo "${cosign_sha256}  /usr/local/bin/cosign" | sha256sum -c - \
    && chmod +x /usr/local/bin/cosign

FROM alpine:3.24.3
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
