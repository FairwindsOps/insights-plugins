# Build the manager binary
FROM golang:1.17 as builder

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.mod
# COPY go.sum go.sum

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

COPY cmd cmd
# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -o right-sizer cmd/main.go

FROM alpine:3.14
RUN apk --no-cache add ca-certificates

COPY --from=builder /workspace/right-sizer /usr/local/bin/

ENTRYPOINT []
CMD ["right-sizer"]
