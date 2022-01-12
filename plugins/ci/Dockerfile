FROM golang:1.17 as build-env
WORKDIR /go/src/github.com/fairwindsops/insights-plugins/


ENV GO111MODULE=on
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64
ENV trivyVersion=0.22.0
ENV polarisVersion=1.2.1
ENV plutoVersion=3.4.1

COPY ci/go.mod ci/
COPY ci/go.sum ci/

COPY trivy trivy
COPY opa opa

WORKDIR /go/src/github.com/fairwindsops/insights-plugins/ci

RUN go mod download

COPY ci/pkg pkg
COPY ci/cmd cmd
RUN go build ./cmd/insights-ci

RUN curl -L https://github.com/aquasecurity/trivy/releases/download/v${trivyVersion}/trivy_${trivyVersion}_Linux-64bit.tar.gz > trivy.tar.gz && tar -xvf trivy.tar.gz && mv ./trivy /usr/local/bin/trivy && rm trivy.tar.gz
RUN curl -L "https://github.com/FairwindsOps/polaris/releases/download/$polarisVersion/polaris_${polarisVersion}_Linux_amd64.tar.gz" > polaris.tar.gz && tar -xvf polaris.tar.gz && chmod +x polaris && rm polaris.tar.gz && mv ./polaris /usr/local/bin/polaris
RUN curl -L "https://github.com/FairwindsOps/pluto/releases/download/v$plutoVersion/pluto_${plutoVersion}_linux_amd64.tar.gz" > pluto.tar.gz && tar -xvf pluto.tar.gz && chmod +x pluto && rm pluto.tar.gz && mv ./pluto /usr/local/bin/pluto
RUN DESIRED_VERSION=3.2.1 curl https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 | bash
FROM alpine:3.15
WORKDIR /insights
COPY --from=build-env /go/src/github.com/fairwindsops/insights-plugins/ci/insights-ci /usr/local/bin/insights-ci
COPY --from=build-env /usr/local/bin/trivy /usr/local/bin/trivy
COPY --from=build-env /usr/local/bin/polaris /usr/local/bin/polaris
COPY --from=build-env /usr/local/bin/helm /usr/local/bin/helm
COPY --from=build-env /usr/local/bin/pluto /usr/local/bin/pluto

RUN apk add --no-cache skopeo git libc6-compat


CMD ["insights-ci"]
