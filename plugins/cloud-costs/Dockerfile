FROM alpine:3.19
ARG TARGETARCH
ARG TARGETOS

RUN apk -U upgrade
RUN apk update && apk upgrade
RUN apk add jq bash curl moreutils
RUN apk add unzip

# install awscli using curl https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip
RUN curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" && \
    unzip awscliv2.zip && \
    ./aws/install && \
    rm -rf awscliv2.zip ./aws

# install python
RUN apk add python3 py3-pip

# install google-cloud-cli using curl https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-sdk-360.0.0-linux-x86_64.tar.gz
RUN curl "https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-sdk-360.0.0-linux-x86_64.tar.gz" -o "google-cloud-sdk-360.0.0-linux-x86_64.tar.gz" && \
    tar -xvf google-cloud-sdk-360.0.0-linux-x86_64.tar.gz && \
    ./google-cloud-sdk/install.sh && \
    rm -rf google-cloud-sdk-360.0.0-linux-x86_64.tar.gz ./google-cloud-sdk

COPY cloud-costs.sh cloud-costs.sh

CMD ["cloud-costs.sh"]
