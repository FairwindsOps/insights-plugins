FROM alpine:3.20
ARG TARGETARCH
ARG TARGETOS
RUN apk -U upgrade
RUN apk add curl
RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/v1.19.6/bin/${TARGETOS}/${TARGETARCH}/kubectl && chmod +x ./kubectl && mv ./kubectl /usr/local/bin/kubectl
USER 10010
ENTRYPOINT [ "kubectl" ]
CMD [ "--help" ]
