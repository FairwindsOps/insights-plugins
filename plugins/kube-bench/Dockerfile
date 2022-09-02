# Build the manager binary
FROM golang:1.18 as builder
ARG TARGETARCH
ARG TARGETOS

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

COPY pkg pkg
# Build
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GO111MODULE=on go build -o kube-bench-daemon pkg/main.go


FROM aquasec/kube-bench:v0.6.8
RUN apk --no-cache add ca-certificates
RUN apk -U upgrade

COPY --from=builder /workspace/kube-bench-daemon /usr/local/bin/

ENTRYPOINT []
CMD ["kube-bench-daemon"]
