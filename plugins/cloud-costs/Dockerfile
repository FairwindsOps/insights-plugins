FROM alpine:3.20

ARG TARGETARCH
ARG TARGETOS

RUN apk -U upgrade
RUN apk update && apk upgrade
RUN apk add jq bash curl moreutils coreutils
RUN apk add unzip

ENV google_cloud_sdk_version=494.0.0

RUN apk add python3
RUN apk add --no-cache aws-cli

RUN if [ "${TARGETARCH}" = "arm64" ]; then \
        googleArch="arm"; \
    else \
        googleArch="x86_64"; \
    fi && \
    curl https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-sdk-${google_cloud_sdk_version}-${TARGETOS}-${googleArch}.tar.gz -o "google-cloud-sdk.tar.gz" && \
    tar -xvf google-cloud-sdk.tar.gz && \
    ./google-cloud-sdk/install.sh && \
    ./google-cloud-sdk/bin/gcloud components install bq && \
    rm -rf /google-cloud-sdk/lib/googlecloudsdk/third_party /google-cloud-sdk/platform/gsutil/third_party /google-cloud-sdk/lib/googlecloudsdk/generated_clients/apis/appengine && \
    rm -rf google-cloud-sdk.tar.gz && \
    rm -rf /google-cloud-sdk/bin/gcloud-crc32c
    

COPY cloud-costs.sh cloud-costs.sh

CMD ["cloud-costs.sh"]
