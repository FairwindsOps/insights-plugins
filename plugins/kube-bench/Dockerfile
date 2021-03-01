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
# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -o kube-bench-daemon pkg/main.go


FROM aquasec/kube-bench@sha256:bdb9dbd323bfef9f53f7acc7c9446f4bd896ff927acad3021c54b341a17be297
RUN apk --no-cache add ca-certificates

COPY --from=builder /workspace/kube-bench-daemon /usr/local/bin/

ENTRYPOINT []
CMD ["kube-bench-daemon"]
