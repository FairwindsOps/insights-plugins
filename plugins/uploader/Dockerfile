FROM alpine:3.17
ARG TARGETARCH
ARG TARGETOS
WORKDIR /usr/local/bin
RUN apk -U upgrade
RUN apk --no-cache add ca-certificates curl

ENV kubectlVersion=1.27.0-alpha.1
RUN curl -LO https://dl.k8s.io/release/$kubectlVersion/bin/${TARGETOS}/${TARGETARCH}/kubectl
RUN chmod +x ./kubectl && mv ./kubectl /usr/local/bin/kubectl

COPY download.sh .
COPY uploader.sh .
COPY version.txt .
ENTRYPOINT ["uploader.sh"]
