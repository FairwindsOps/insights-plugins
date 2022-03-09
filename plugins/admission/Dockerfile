FROM golang:1.17 as build-env
WORKDIR /go/src/github.com/fairwindsops/insights-plugins/


ENV GO111MODULE=on
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64
ENV GOFLAGS=-mod=mod

COPY opa opa

WORKDIR /go/src/github.com/fairwindsops/insights-plugins/admission

COPY ./admission/go.* ./
RUN go mod download

RUN go get -u github.com/gobuffalo/packr/v2/packr2

COPY ./admission/pkg pkg
COPY ./admission/cmd cmd
COPY ./admission/version.txt ./
RUN packr2 build /go/pkg/mod/github.com/fairwindsops/polaris*/main.go
RUN go build ./cmd/admission

FROM alpine:3.15
WORKDIR /insights
COPY --from=build-env /go/src/github.com/fairwindsops/insights-plugins/admission/admission /usr/local/bin/insights-admission

CMD ["insights-admission"]
