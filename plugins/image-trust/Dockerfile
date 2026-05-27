FROM alpine:3.23.4 AS downloader
ARG TARGETARCH
ENV COSIGN_VERSION=3.0.6
RUN apk --no-cache add ca-certificates curl \
    && if [ "${TARGETARCH}" = "amd64" ]; then cosign_arch="amd64"; else cosign_arch="${TARGETARCH}"; fi \
    && curl -L "https://github.com/sigstore/cosign/releases/download/v${COSIGN_VERSION}/cosign-linux-${cosign_arch}" -o /usr/local/bin/cosign \
    && chmod +x /usr/local/bin/cosign

FROM alpine:3.23.4
WORKDIR /usr/local/bin
RUN apk --no-cache add ca-certificates \
    && apk --no-cache add --upgrade libcrypto3 libssl3 zlib

RUN addgroup -S insights && adduser -u 1200 -S insights -G insights
USER 1200
COPY image-trust .
COPY --from=downloader /usr/local/bin/cosign /usr/local/bin/cosign
COPY report.sh /opt/app/report.sh

WORKDIR /opt/app

CMD ["sh", "./report.sh"]
