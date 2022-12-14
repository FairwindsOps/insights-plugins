FROM alpine:3.17
ARG TARGETARCH
ARG TARGETOS
WORKDIR /usr/local/bin
RUN apk -U upgrade
RUN apk --no-cache add ca-certificates curl

RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/${TARGETOS}/${TARGETARCH}/kubectl && chmod +x ./kubectl && mv ./kubectl /usr/local/bin/kubectl

COPY download.sh .
COPY uploader.sh .
COPY version.txt .
ENTRYPOINT ["uploader.sh"]
