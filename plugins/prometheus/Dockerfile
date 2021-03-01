# Build the manager binary
FROM golang:1.15.1 as builder

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

COPY pkg pkg
COPY cmd cmd
# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -o prometheus-collector ./cmd/prometheus-collector


FROM alpine:3.13

COPY --from=builder /workspace/prometheus-collector /usr/local/bin/

CMD ["prometheus-collector"]
