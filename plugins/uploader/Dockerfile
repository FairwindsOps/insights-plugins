FROM alpine:3.20
ARG TARGETARCH
ARG TARGETOS
WORKDIR /usr/local/bin
RUN apk -U upgrade
RUN apk --no-cache add bash ca-certificates curl

# curl -L -s https://dl.k8s.io/release/stable.txt
ENV kubectlVersion=v1.30.0
RUN curl -LO https://dl.k8s.io/release/$kubectlVersion/bin/${TARGETOS}/${TARGETARCH}/kubectl
RUN chmod +x ./kubectl && mv ./kubectl /usr/local/bin/kubectl

COPY download.sh .
COPY uploader.sh .
COPY version.txt .
ENTRYPOINT ["uploader.sh"]
