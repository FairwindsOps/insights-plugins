FROM alpine:3.22
WORKDIR /usr/local/bin

RUN apk -U upgrade
RUN apk --no-cache add ca-certificates

# Install kubectl
ENV kubectlVersion=1.33.4
RUN curl -LO https://dl.k8s.io/release/$kubectlVersion/bin/${TARGETOS}/${TARGETARCH}/kubectl

# Switch to non-root user
USER 1200
COPY kyverno-policy-sync .

WORKDIR /opt/app

# Default command
CMD ["kyverno-policy-sync"]
