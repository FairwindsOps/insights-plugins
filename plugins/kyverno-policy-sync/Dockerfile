FROM golang:1.24.6-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o kyverno-policy-sync ./cmd/main.go

# Final stage
FROM alpine:3.22

# Install runtime dependencies
RUN apk -U upgrade && \
    apk --no-cache add ca-certificates tzdata

# Install Kyverno CLI
RUN wget -O kyverno https://github.com/kyverno/kyverno/releases/latest/download/kyverno-cli_linux_amd64 && \
    chmod +x kyverno && \
    mv kyverno /usr/local/bin/

# Create non-root user
RUN adduser -D -s /bin/sh -u 1200 kyverno

# Copy binary from builder stage
COPY --from=builder /app/kyverno-policy-sync /usr/local/bin/kyverno-policy-sync

# Set working directory
WORKDIR /opt/app

# Switch to non-root user
USER 1200

# Default command
CMD ["kyverno-policy-sync"]
