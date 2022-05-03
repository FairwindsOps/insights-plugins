FROM golang:1.17 as build-env
WORKDIR /go/src/github.com/fairwindsops/insights-plugins/plugins/


ENV GO111MODULE=on
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64
ENV GOFLAGS=-mod=mod

COPY opa opa

WORKDIR /go/src/github.com/fairwindsops/insights-plugins/plugins/admission

COPY ./admission/go.* ./
RUN go mod download

RUN go get -u github.com/gobuffalo/packr/v2/packr2

COPY ./admission/version.txt .
COPY ./admission/embed.go .
COPY ./admission/pkg pkg
COPY ./admission/cmd cmd

RUN packr2 build /go/pkg/mod/github.com/fairwindsops/polaris*/main.go
RUN go build ./cmd/admission

FROM alpine:20220328
WORKDIR /insights
COPY --from=build-env /go/src/github.com/fairwindsops/insights-plugins/plugins/admission/admission /usr/local/bin/insights-admission

CMD ["insights-admission"]
