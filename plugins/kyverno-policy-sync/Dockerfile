FROM alpine:3.22
WORKDIR /usr/local/bin

RUN apk -U upgrade
RUN apk --no-cache add ca-certificates

# Install kubectl
ARG KUBECTL_VERSION=v1.33.4
RUN wget -O /usr/local/bin/kubectl https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl && \
    chmod +x /usr/local/bin/kubectl

# Switch to non-root user
USER 1200
COPY kyverno-policy-sync .

WORKDIR /opt/app

# Default command
CMD ["kyverno-policy-sync"]
