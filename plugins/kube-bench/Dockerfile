FROM aquasec/kube-bench:v0.15.0

ARG TARGETARCH
ARG TARGETOS
RUN apk --no-cache add bash ca-certificates curl \
    && apk --no-cache add --upgrade libcrypto3 libssl3 zlib

RUN rm /usr/local/bin/kubectl
ENV kubectlVersion=1.35.3
RUN curl -LO https://dl.k8s.io/release/v${kubectlVersion}/bin/${TARGETOS}/${TARGETARCH}/kubectl
RUN chmod +x ./kubectl && mv ./kubectl /usr/local/bin/kubectl

COPY kube-bench-daemon /usr/local/bin/

ENTRYPOINT []
CMD ["kube-bench-daemon"]
