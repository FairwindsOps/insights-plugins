FROM alpine:3.20 AS downloader
ARG TARGETARCH
ARG TARGETOS
ENV trivyVersion=0.56.2
RUN apk update && apk add curl

ENV kubectlVersion=1.31.0
RUN curl -LO https://dl.k8s.io/release/$kubectlVersion/bin/${TARGETOS}/${TARGETARCH}/kubectl
RUN chmod +x ./kubectl && mv ./kubectl /usr/local/bin/kubectl

RUN if [ "${TARGETARCH}" = "amd64" ] ; then trivyArch="64bit"; else trivyArch="${TARGETARCH}"; fi && \
  curl -L https://github.com/aquasecurity/trivy/releases/download/v${trivyVersion}/trivy_${trivyVersion}_${TARGETOS}-${trivyArch}.tar.gz > trivy.tar.gz && tar -xvf trivy.tar.gz && mv ./trivy /usr/local/bin/trivy && rm trivy.tar.gz

FROM alpine:3.20
WORKDIR /trivy
RUN apk -U upgrade

# for gcloud
RUN apk --no-cache add bash curl python3 libc6-compat ca-certificates

ENV CLOUD_SDK_VERSION=497.0.0

ENV MAPPED_ARCH=""
RUN case "$TARGETARCH" in \
  "amd64" | "x86_64" | "x64") \
  MAPPED_ARCH="x86_64" ;; \
  "arm64" | "aarch64" | "armv8" | "armv8l") \
  MAPPED_ARCH="arm" ;; \
  "arm" | "armhf" | "armel" | "armv6" | "armv7" | "armv7l") \
  MAPPED_ARCH="arm" ;; \
  "386" | "i386" | "i486" | "i586" | "i686") \
  MAPPED_ARCH="x86" ;; \
  *) \
  echo "Unsupported architecture: $TARGETARCH" && exit 1 ;; \
  esac

RUN curl -LO https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-cli-${CLOUD_SDK_VERSION}-${TARGETOS}-${MAPPED_ARCH}.tar.gz && \ 
  tar -xvf google-cloud-cli-${CLOUD_SDK_VERSION}-${TARGETOS}-${MAPPED_ARCH}.tar.gz && \ 
  rm google-cloud-cli-${CLOUD_SDK_VERSION}-${TARGETOS}-${MAPPED_ARCH}.tar.gz && \ 
  google-cloud-sdk/install.sh --quiet --path-update=false

ENV PATH=$PATH:/trivy/google-cloud-sdk/bin

COPY scan ./scan
COPY --from=downloader /usr/local/bin/kubectl /usr/local/bin/kubectl
COPY --from=downloader /usr/local/bin/trivy /usr/local/bin/trivy

RUN TRIVY_CACHE_DIR=/var/tmp trivy image \ 
  --download-db-only \ 
  --db-repository "ghcr.io/aquasecurity/trivy-db:2","public.ecr.aws/aquasecurity/trivy-db:2","docker.io/aquasec/trivy-db:2" \ 
  --java-db-repository "ghcr.io/aquasecurity/trivy-java-db:1","public.ecr.aws/aquasecurity/trivy-java-db:1","docker.io/aquasec/trivy-java-db:1"

ENV CLOUDSDK_CONFIG=/tmp/gcloud
ENV HOME=/
RUN mkdir $HOME/.docker

RUN apk add --no-cache skopeo

COPY report.sh ./

# remove all unused components that have HIGH/CRITICAL vulnerabilities
RUN rm -rf /trivy/google-cloud-sdk/platform/gsutil/third_party
RUN rm -rf /trivy/google-cloud-sdk/bin/gcloud-crc32c
RUN rm -f /trivy/google-cloud-sdk/lib/third_party/google/auth/crypt/__pycache__/_python_rsa.cpython-312.pyc
RUN rm -f /trivy/google-cloud-sdk/lib/third_party/oauth2client/__pycache__/_pure_python_crypt.cpython-312.pyc

CMD ./report.sh
