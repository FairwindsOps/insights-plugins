FROM alpine:3.23.4
ARG TARGETARCH
ARG TARGETOS
RUN apk add --no-cache ca-certificates curl \
    && apk --no-cache add --upgrade libcrypto3 libssl3 zlib
ENV kubectlVersion=1.35.3
RUN curl -LO https://dl.k8s.io/release/v${kubectlVersion}/bin/${TARGETOS}/${TARGETARCH}/kubectl
RUN chmod +x ./kubectl && mv ./kubectl /usr/local/bin/kubectl

USER 10010
ENTRYPOINT [ "kubectl" ]
CMD [ "--help" ]
