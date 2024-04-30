FROM alpine:3.19
ARG TARGETARCH
ARG TARGETOS

RUN apk -U upgrade
RUN apk update && apk upgrade
RUN apk add jq bash curl moreutils coreutils
RUN apk add unzip

RUN apk add python3

RUN curl "https://awscli.amazonaws.com/awscli-exe-l${TARGETOS}_${TARGETARCH}.zip" -o "awscliv2.zip" && \
    unzip awscliv2.zip && \
    ./aws/install && \
    rm -rf awscliv2.zip ./aws

RUN curl "https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-sdk-473.0.0-${TARGETOS}_${TARGETARCH}.tar.gz" -o "google-cloud-sdk-473.0.0-${TARGETOS}_${TARGETARCH}.tar.gz" && \
    tar -xvf google-cloud-sdk-473.0.0-${TARGETOS}_${TARGETARCH}.tar.gz && \
    ./google-cloud-sdk/install.sh && \
    ./google-cloud-sdk/bin/gcloud components install bq && \
    rm -rf google-cloud-sdk-473.0.0-${TARGETOS}_${TARGETARCH}.tar.gz 

RUN /google-cloud-sdk/bin/bq --help

#COPY cloud-costs.sh cloud-costs.sh
#CMD ["cloud-costs.sh"]
