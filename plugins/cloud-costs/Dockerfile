FROM mcr.microsoft.com/azure-cli:2.82.0-azurelinux3.0

ARG TARGETARCH
ARG TARGETOS

ENV CLOUD_SDK_VERSION=554.0.0

# Install base dependencies (python3-pip needed to upgrade Azure CLI Python deps)
RUN tdnf -y update && \
    tdnf -y install jq curl bash coreutils unzip tar ca-certificates python3-pip && \
    tdnf clean all

# Upgrade Azure CLI Python deps to fix known CVEs (urllib3, wheel, setuptools/jaraco.context).
# Skip azure-core: upgrading it via --target overwrites the azure namespace and breaks azure.cli.
RUN python3 -m pip install --no-cache-dir --target /usr/lib/az/lib/python3.12/site-packages --upgrade \
    'urllib3>=2.6.1' \
    'wheel>=0.46.2' \
    'setuptools>=70'

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
    rm -rf /google-cloud-sdk/platform/bundledpythonunix/lib/python3.13/site-packages/setuptools/_vendor/jaraco*; \
    /google-cloud-sdk/platform/bundledpythonunix/bin/python3 -m pip install --no-cache-dir --upgrade 'wheel>=0.46.2'

ENV PATH=$PATH:/google-cloud-sdk/bin

# Install script on PATH and use as entrypoint
COPY cloud-costs.sh /usr/local/bin/cloud-costs.sh
RUN chmod +x /usr/local/bin/cloud-costs.sh

ENTRYPOINT ["cloud-costs.sh"]
