FROM golang:1.17 as build-env
WORKDIR /go/src/github.com/fairwindsops/insights-plugins/trivy/


ENV GO111MODULE=on
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY pkg ./pkg
COPY cmd ./cmd
RUN go build -a -o scan ./cmd/trivy

RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/`curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt`/bin/linux/amd64/kubectl && chmod +x ./kubectl && mv ./kubectl /usr/local/bin/kubectl

RUN curl -L https://github.com/aquasecurity/trivy/releases/download/v0.21.3/trivy_0.21.3_Linux-64bit.tar.gz > trivy.tar.gz && tar -xvf trivy.tar.gz && mv ./trivy /usr/local/bin/trivy && rm trivy.tar.gz


FROM alpine:3.15
WORKDIR /trivy
COPY --from=build-env /go/src/github.com/fairwindsops/insights-plugins/trivy/scan ./scan
COPY --from=build-env /usr/local/bin/kubectl /usr/local/bin/kubectl
COPY --from=build-env /usr/local/bin/trivy /usr/local/bin/trivy

ENV HOME=/
RUN mkdir $HOME/.docker

RUN apk add --no-cache skopeo

COPY report.sh ./
CMD ./report.sh
