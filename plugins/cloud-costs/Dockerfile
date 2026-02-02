FROM python:3-alpine3.22

ARG TARGETARCH
ARG TARGETOS

RUN apk -U upgrade
RUN apk update && apk upgrade
RUN apk add jq 
RUN apk add curl
RUN apk add bash
RUN apk add moreutils
RUN apk add coreutils
RUN apk add unzip

ENV CLOUD_SDK_VERSION=554.0.0

RUN apk add --no-cache aws-cli

# Install Google Cloud SDK
RUN if [ "${TARGETARCH}" = "arm64" ]; then \
        googleArch="arm"; \
    else \
        googleArch="x86_64"; \
    fi && \
    curl -LO https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-cli-${CLOUD_SDK_VERSION}-linux-${googleArch}.tar.gz && \
    tar -xvf google-cloud-cli-${CLOUD_SDK_VERSION}-linux-${googleArch}.tar.gz && \
    rm google-cloud-cli-${CLOUD_SDK_VERSION}-linux-${googleArch}.tar.gz && \
    google-cloud-sdk/install.sh --quiet --path-update=false && \
    rm -rf /google-cloud-sdk/platform/gsutil/third_party && \
    rm -rf /google-cloud-sdk/platform/bundledpythonunix/lib/python3.13/site-packages/setuptools/_vendor/jaraco*

ENV PATH=$PATH:/google-cloud-sdk/bin

COPY cloud-costs.sh cloud-costs.sh

CMD ["cloud-costs.sh"]
