FROM alpine:3.19

ARG TARGETARCH="arm64"
ARG TARGETOS

RUN apk -U upgrade
RUN apk update && apk upgrade
RUN apk add jq bash curl moreutils coreutils
RUN apk add unzip

RUN apk add python3

RUN if [ "arm64" = "arm64" ] ; then awsArch="aarch64"; else awsArch="x86_64"; fi && \
    curl https://awscli.amazonaws.com/awscli-exe-${TARGETOS}-${awsArch}.zip > awscliv2.zip && \
    unzip awscliv2.zip && \
    ./aws/install -i /usr/local/aws-cli -b /usr/local/bin && \
    rm -rf awscliv2.zip ./aws

RUN if [ "arm64" = "arm64" ] ; then googleArch="arm"; else googleArch="x86_64"; fi && \
    curl "https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-sdk-400.0.0-${TARGETOS}-${googleArch}.tar.gz" -o "google-cloud-sdk-400.0.0.tar.gz" && \
    tar -xvf google-cloud-sdk-400.0.0.tar.gz && \
    ./google-cloud-sdk/install.sh && \
    rm -rf google-cloud-sdk-400.0.0.tar.gz

RUN ./google-cloud-sdk/bin/gcloud components install bq


#RUN /usr/local/aws-cli/aws --version

COPY cloud-costs.sh cloud-costs.sh

CMD ["cloud-costs.sh"]
