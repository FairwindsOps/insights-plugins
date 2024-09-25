FROM alpine:3.20
ARG TARGETARCH
ARG TARGETOS
RUN apk -U upgrade
RUN apk add curl
ENV kubectlVersion=1.31.0
RUN curl -LO https://dl.k8s.io/release/$kubectlVersion/bin/${TARGETOS}/${TARGETARCH}/kubectl
RUN chmod +x ./kubectl && mv ./kubectl /usr/local/bin/kubectl

USER 10010
ENTRYPOINT [ "kubectl" ]
CMD [ "--help" ]
