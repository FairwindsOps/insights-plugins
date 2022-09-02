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
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GO111MODULE=on go build -a -o workload pkg/*.go


FROM alpine:3.16
WORKDIR /usr/local/bin
RUN apk -U upgrade
RUN apk --no-cache add ca-certificates

RUN addgroup -S insights && adduser -u 1200 -S insights -G insights
USER 1200
COPY --from=builder /workspace/workload .

WORKDIR /opt/app

CMD ["workload"]
