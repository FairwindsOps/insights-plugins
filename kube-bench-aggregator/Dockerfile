# Build the manager binary
FROM golang:1.14.3 as builder

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

COPY pkg pkg
# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -o kube-bench pkg/main.go


FROM alpine:3.11
WORKDIR /usr/local/bin
RUN apk --no-cache add ca-certificates

RUN addgroup -S insights && adduser -u 1200 -S insights -G insights
USER 1200
COPY --from=builder /workspace/kube-bench .

WORKDIR /opt/app

CMD ["kube-bench"]
