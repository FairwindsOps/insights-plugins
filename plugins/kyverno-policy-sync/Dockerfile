FROM alpine:3.23
ARG TARGETARCH
ARG TARGETOS
WORKDIR /usr/local/bin
RUN apk update && apk -U upgrade
RUN apk --no-cache add ca-certificates curl

# Install kubectl
ENV kubectlVersion=1.35.3
RUN curl -LO https://dl.k8s.io/release/v${kubectlVersion}/bin/${TARGETOS}/${TARGETARCH}/kubectl
RUN chmod +x ./kubectl && mv ./kubectl /usr/local/bin/kubectl

# Switch to non-root user
USER 1200
COPY kyverno-policy-sync .

WORKDIR /opt/app

# Default command
CMD ["kyverno-policy-sync"]
