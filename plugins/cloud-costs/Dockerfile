FROM alpine:3.19

ARG TARGETARCH
ARG TARGETOS
ARG awsArch

RUN apk -U upgrade
RUN apk update && apk upgrade
RUN apk add jq bash curl moreutils coreutils
RUN apk add unzip

RUN apk add python3

RUN if [ "${TARGETARCH}" = "amd64" ] ; then awsArch="aarch64"; else trivyArch="${TARGETARCH}"; fi && \
RUN curl "https://awscli.amazonaws.com/awscli-exe-${TARGETOS}_${awsArch}.zip" -o "awscliv2.zip" && \
    unzip awscliv2.zip && \
    ./aws/install && \
    rm -rf awscliv2.zip ./aws

RUN curl "https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-sdk-400.0.0-${TARGETOS}_${TARGETARCH}.tar.gz" -o "google-cloud-sdk-400.0.0-${TARGETOS}_${TARGETARCH}.tar.gz" && \
    tar -xvf google-cloud-sdk-400.0.0-${TARGETOS}_${TARGETARCH}.tar.gz && \
    ./google-cloud-sdk/install.sh && \
    rm -rf google-cloud-sdk-400.0.0-${TARGETOS}_${TARGETARCH}.tar.gz 

RUN ./google-cloud-sdk/bin/gcloud components install bq

COPY cloud-costs.sh cloud-costs.sh

CMD ["cloud-costs.sh"]
