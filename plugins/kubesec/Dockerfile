FROM alpine:3.21
ARG TARGETARCH
ARG TARGETOS

RUN apk -U upgrade
RUN echo "TODO: this line just removes a cached layer"
# Is curl only used for build? Can we move to a multistage? Can we remove some of these other packages?
RUN apk update && apk upgrade
RUN apk add curl python3 jq yq tar go bash py3-pip

ENV kubectlVersion=1.31.0
RUN curl -LO https://dl.k8s.io/release/$kubectlVersion/bin/${TARGETOS}/${TARGETARCH}/kubectl
RUN chmod +x ./kubectl && mv ./kubectl /usr/local/bin/kubectl

ENV GOPATH=/home/ubuntu/go/
RUN curl -L "https://github.com/controlplaneio/kubesec/releases/download/v2.14.2/kubesec_${TARGETOS}_${TARGETARCH}.tar.gz" > kubesec.tar.gz && tar -xvf kubesec.tar.gz && mv kubesec /usr/bin/ && rm kubesec.tar.gz

COPY main.sh .

USER 1000
CMD ["main.sh"]

