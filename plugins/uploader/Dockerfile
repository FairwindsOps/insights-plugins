FROM alpine:3.23.4
ARG TARGETARCH
ARG TARGETOS
WORKDIR /usr/local/bin
RUN apk --no-cache add bash ca-certificates curl \
    && apk --no-cache add --upgrade libcrypto3 libssl3 zlib

# curl -L -s https://dl.k8s.io/release/stable.txt
ENV kubectlVersion=v1.35.3
RUN curl -LO https://dl.k8s.io/release/${kubectlVersion}/bin/${TARGETOS}/${TARGETARCH}/kubectl
RUN chmod +x ./kubectl && mv ./kubectl /usr/local/bin/kubectl

COPY download.sh .
COPY uploader.sh .
COPY version.txt .
ENTRYPOINT ["uploader.sh"]
