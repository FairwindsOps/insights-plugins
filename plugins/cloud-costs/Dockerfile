FROM mcr.microsoft.com/azure-cli:2.82.0-azurelinux3.0

ARG TARGETARCH
ARG TARGETOS

ENV CLOUD_SDK_VERSION=554.0.0

# Install base dependencies
RUN tdnf -y update && \
    tdnf -y install jq curl bash coreutils unzip tar ca-certificates && \
    tdnf clean all

# Install AWS CLI v2
RUN set -eux; \
    case "${TARGETARCH}" in \
      amd64) awsArch="x86_64" ;; \
      arm64) awsArch="aarch64" ;; \
      *) echo "unsupported TARGETARCH=${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    curl -fsSL "https://awscli.amazonaws.com/awscli-exe-linux-${awsArch}.zip" -o awscliv2.zip; \
    unzip -q awscliv2.zip; \
    ./aws/install -i /usr/local/aws-cli -b /usr/local/bin; \
    rm -rf aws awscliv2.zip

# Install Google Cloud SDK
RUN set -eux; \
    if [ "${TARGETARCH}" = "arm64" ]; then \
        googleArch="arm"; \
    else \
        googleArch="x86_64"; \
    fi; \
    curl -fsSLO "https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-cli-${CLOUD_SDK_VERSION}-linux-${googleArch}.tar.gz"; \
    tar -xvf "google-cloud-cli-${CLOUD_SDK_VERSION}-linux-${googleArch}.tar.gz"; \
    rm "google-cloud-cli-${CLOUD_SDK_VERSION}-linux-${googleArch}.tar.gz"; \
    /google-cloud-sdk/install.sh --quiet --path-update=false; \
    rm -rf /google-cloud-sdk/platform/gsutil/third_party; \
    rm -rf /google-cloud-sdk/platform/bundledpythonunix/lib/python3.13/site-packages/setuptools/_vendor/jaraco*

ENV PATH=$PATH:/google-cloud-sdk/bin

# Azure CLI writes config to AZURE_CONFIG_DIR; use writable path (root fs is read-only in K8s)
ENV AZURE_CONFIG_DIR=/tmp/.azure

# Chart runs container with command ./cloud-costs.sh, so script must be in working dir
WORKDIR /app
COPY cloud-costs.sh .
RUN chmod +x cloud-costs.sh

CMD ["./cloud-costs.sh"]
