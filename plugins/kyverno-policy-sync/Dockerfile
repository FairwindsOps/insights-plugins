FROM alpine:3.22
WORKDIR /usr/local/bin
# Install runtime dependencies
ARG KYVERNO_VERSION=v1.15.2
RUN apk -U upgrade && \
    apk --no-cache add ca-certificates tzdata
# Install Kyverno CLI
RUN wget -O /usr/local/bin/kyverno.tar.gz https://github.com/kyverno/kyverno/releases/download/${KYVERNO_VERSION}/kyverno-cli_${KYVERNO_VERSION}_linux_x86_64.tar.gz && \
    tar -xf /usr/local/bin/kyverno.tar.gz -C /usr/local/bin && \
    chmod +x /usr/local/bin/kyverno && \
    rm /usr/local/bin/kyverno.tar.gz


# Set working directory and switch to non-root user
USER 1200
COPY kyverno-policy-sync .

WORKDIR /opt/app

# Default command
CMD ["kyverno-policy-sync"]
