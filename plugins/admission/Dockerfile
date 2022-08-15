# syntax = docker/dockerfile:1.3
# Minimally dockerfile:2.1 is required by RUN --mount=type=cache
# Ref: https://github.com/moby/buildkit/blob/master/frontend/dockerfile/docs/syntax.md
FROM golang:1.18 as build-env
WORKDIR /go/src/github.com/fairwindsops/insights-plugins/plugins/


ENV GO111MODULE=on
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64
ENV GOFLAGS=-mod=mod

COPY opa opa

WORKDIR /go/src/github.com/fairwindsops/insights-plugins/plugins/admission

COPY ./admission/go.* ./
RUN --mount=type=cache,target=/root/.cache/go-build go mod download

RUN --mount=type=cache,target=/root/.cache/go-build go install github.com/gobuffalo/packr/v2/packr2@latest

COPY ./admission/version.txt .
COPY ./admission/embed.go .
COPY ./admission/pkg pkg
COPY ./admission/cmd cmd

RUN --mount=type=cache,target=/root/.cache/go-build packr2 build /go/pkg/mod/github.com/fairwindsops/polaris*/main.go
RUN --mount=type=cache,target=/root/.cache/go-build go build ./cmd/admission

FROM alpine:3.16
WORKDIR /insights
COPY --from=build-env /go/src/github.com/fairwindsops/insights-plugins/plugins/admission/admission /usr/local/bin/insights-admission

CMD ["insights-admission"]
