FROM alpine:3.22
WORKDIR /usr/local/bin

RUN apk -U upgrade
RUN apk --no-cache add ca-certificates
# Install Kyverno CLI
ARG KYVERNO_VERSION=v1.15.2
RUN wget -O /usr/local/bin/kyverno.tar.gz https://github.com/kyverno/kyverno/releases/download/${KYVERNO_VERSION}/kyverno-cli_${KYVERNO_VERSION}_linux_x86_64.tar.gz && \
    tar -xf /usr/local/bin/kyverno.tar.gz -C /usr/local/bin && \
    chmod +x /usr/local/bin/kyverno && \
    rm /usr/local/bin/kyverno.tar.gz

# Create output directory for temporary files
RUN mkdir -p /output/tmp && chown -R 1200:1200 /output

# Switch to non-root user
USER 1200
COPY kyverno-policy-sync .

WORKDIR /opt/app

# Default command
CMD ["kyverno-policy-sync"]
