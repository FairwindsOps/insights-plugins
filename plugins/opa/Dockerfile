# Build the manager binary
FROM golang:1.17 as builder

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
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o fw-opa cmd/opa/main.go


FROM alpine:20220715
WORKDIR /usr/local/bin
RUN apk --no-cache add ca-certificates

USER 1200
COPY --from=builder /workspace/fw-opa .

WORKDIR /opt/app

CMD ["fw-opa"]
