FROM golang:1.18 as build-env
ARG TARGETARCH
ARG TARGETOS
WORKDIR /go/src/github.com/fairwindsops/insights-plugins/plugins/trivy/


ENV GO111MODULE=on
ENV CGO_ENABLED=0
ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH
ENV trivyVersion=0.30.4

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY pkg ./pkg
COPY cmd ./cmd
RUN go build -a -o scan ./cmd/trivy

RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/`curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt`/bin/${TARGETOS}/${TARGETARCH}/kubectl && chmod +x ./kubectl && mv ./kubectl /usr/local/bin/kubectl

RUN if [ "${TARGETARCH}" = "amd64" ] ; then trivyArch="64bit"; else trivyArch="${TARGETARCH}"; fi && \
  curl -L https://github.com/aquasecurity/trivy/releases/download/v${trivyVersion}/trivy_${trivyVersion}_${TARGETOS}-${trivyArch}.tar.gz > trivy.tar.gz && tar -xvf trivy.tar.gz && mv ./trivy /usr/local/bin/trivy && rm trivy.tar.gz

FROM alpine:3.16
WORKDIR /trivy
RUN apk -U upgrade
COPY --from=build-env /go/src/github.com/fairwindsops/insights-plugins/plugins/trivy/scan ./scan
COPY --from=build-env /usr/local/bin/kubectl /usr/local/bin/kubectl
COPY --from=build-env /usr/local/bin/trivy /usr/local/bin/trivy

RUN TRIVY_CACHE_DIR=/var/tmp trivy image --download-db-only

ENV HOME=/
RUN mkdir $HOME/.docker

RUN apk add --no-cache skopeo

COPY report.sh ./
CMD ./report.sh
