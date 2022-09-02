# Build the right-sizer controller.
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

COPY cmd cmd
COPY src src
# Build
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GO111MODULE=on go build -o right-sizer cmd/main.go

FROM scratch
USER 1200:1200

COPY --from=builder /workspace/right-sizer /usr/local/bin/

ENTRYPOINT ["right-sizer"]
CMD ["right-sizer"]
