FROM alpine:3.19
ARG TARGETARCH
ARG TARGETOS

RUN apk -U upgrade
RUN apk update && apk upgrade
RUN apk add jq bash curl moreutils coreutils
RUN apk add unzip

# install awscli using curl https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip
RUN curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" && \
    unzip awscliv2.zip && \
    ./aws/install && \
    rm -rf awscliv2.zip ./aws

RUN apk add python3=3.11.9-r0 py3-pip

RUN curl "https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-sdk-373.0.0-linux-x86_64.tar.gz" -o "google-cloud-sdk-373.0.0-linux-x86_64.tar.gz" && \
    tar -xvf google-cloud-sdk-373.0.0-linux-x86_64.tar.gz && \
    ./google-cloud-sdk/install.sh && \
    ./google-cloud-sdk/bin/gcloud components install bq && \    
    rm -rf google-cloud-sdk-360.0.0-linux-x86_64.tar.gz ./google-cloud-sdk


COPY cloud-costs.sh cloud-costs.sh

CMD ["cloud-costs.sh"]
