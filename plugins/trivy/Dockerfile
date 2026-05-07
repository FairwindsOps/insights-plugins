FROM alpine:3.23.4 AS downloader
ARG TARGETARCH
ARG TARGETOS
ENV trivyVersion=0.70.0
RUN apk upgrade --no-cache \
    && apk add --no-cache ca-certificates curl \
    && apk --no-cache add --upgrade libcrypto3 libssl3 zlib musl musl-utils

RUN if [ "${TARGETARCH}" = "amd64" ] ; then trivyArch="64bit"; else trivyArch="${TARGETARCH}"; fi && \
  curl -L https://github.com/aquasecurity/trivy/releases/download/v${trivyVersion}/trivy_${trivyVersion}_${TARGETOS}-${trivyArch}.tar.gz > trivy.tar.gz && tar -xvf trivy.tar.gz && mv ./trivy /usr/local/bin/trivy && rm trivy.tar.gz

FROM python:3-alpine3.23
WORKDIR /trivy
ARG TARGETARCH
# for gcloud
RUN apk upgrade --no-cache \
    && apk --no-cache add bash curl libc6-compat ca-certificates \
    && apk --no-cache add --upgrade libcrypto3 libssl3 zlib musl musl-utils

ENV CLOUD_SDK_VERSION=567.0.0

RUN set -eux; \
    case "${TARGETARCH}" in \
      arm64) googleArch="arm" ;; \
      amd64) googleArch="x86_64" ;; \
      *) echo "unsupported TARGETARCH=${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    curl -fsSLO "https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-cli-${CLOUD_SDK_VERSION}-linux-${googleArch}.tar.gz"; \
    tar -xf "google-cloud-cli-${CLOUD_SDK_VERSION}-linux-${googleArch}.tar.gz"; \
    rm "google-cloud-cli-${CLOUD_SDK_VERSION}-linux-${googleArch}.tar.gz"; \
    google-cloud-sdk/install.sh --quiet --path-update=false; \
    rm -rf /trivy/google-cloud-sdk/platform/gsutil/third_party; \
    rm -rf /trivy/google-cloud-sdk/platform/bundledpythonunix/lib/python3.13/site-packages/docs; \
    rm -rf /trivy/google-cloud-sdk/platform/bundledpythonunix/lib/python3.13/site-packages/pkg_resources/tests; \
    /trivy/google-cloud-sdk/platform/bundledpythonunix/bin/python3 -m pip install --no-cache-dir --upgrade \
      'pyopenssl>=26.0.0' 'cryptography>=44.0.0'

ENV PATH=$PATH:/trivy/google-cloud-sdk/bin

COPY scan ./scan
COPY --from=downloader /usr/local/bin/trivy /usr/local/bin/trivy

RUN TRIVY_CACHE_DIR=/var/tmp trivy image \ 
  --download-db-only \ 
  --db-repository "ghcr.io/aquasecurity/trivy-db:2","public.ecr.aws/aquasecurity/trivy-db:2","docker.io/aquasec/trivy-db:2"

RUN TRIVY_CACHE_DIR=/var/tmp trivy image \ 
  --download-java-db-only \ 
  --java-db-repository "ghcr.io/aquasecurity/trivy-java-db:1","public.ecr.aws/aquasecurity/trivy-java-db:1","docker.io/aquasec/trivy-java-db:1"

ENV CLOUDSDK_CONFIG=/tmp/gcloud
ENV HOME=/
RUN mkdir $HOME/.docker

RUN apk add --no-cache skopeo

COPY report.sh ./
CMD ./report.sh
