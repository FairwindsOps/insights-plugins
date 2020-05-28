FROM golang:1.14.3 AS build-env
WORKDIR /go/src/github.com/fairwindsops/insights-plugins/


ENV GO111MODULE=on
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

COPY ci/go.mod ci/
COPY ci/go.sum ci/

COPY trivy trivy

WORKDIR /go/src/github.com/fairwindsops/insights-plugins/ci

RUN go mod download

COPY ci/pkg pkg
COPY ci/cmd cmd
RUN go build ./cmd/insights-ci

RUN curl -L https://github.com/aquasecurity/trivy/releases/download/v0.6.0/trivy_0.6.0_Linux-64bit.tar.gz > trivy.tar.gz && tar -xvf trivy.tar.gz && mv ./trivy /usr/local/bin/trivy && rm trivy.tar.gz
RUN curl -L "https://github.com/FairwindsOps/polaris/releases/download/0.6.0/polaris_0.6.0_Linux_x86_64.tar.gz" > polaris.tar.gz && tar -xvf polaris.tar.gz && chmod +x polaris && rm polaris.tar.gz && mv ./polaris /usr/local/bin/polaris

FROM alpine:3.11.6
WORKDIR /insights
COPY --from=build-env /go/src/github.com/fairwindsops/insights-plugins/ci/insights-ci ./insights-ci
COPY --from=build-env /usr/local/bin/trivy /usr/local/bin/trivy
COPY --from=build-env /usr/local/bin/polaris /usr/local/bin/polaris

RUN apk add --no-cache skopeo git

