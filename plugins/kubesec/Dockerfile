FROM alpine:20230208
ARG TARGETARCH
ARG TARGETOS

RUN apk -U upgrade
RUN echo "TODO: this line just removes a cached layer"
# Is curl only used for build? Can we move to a multistage? Can we remove some of these other packages?
RUN apk update && apk upgrade
RUN apk add curl python3 jq tar go bash py3-pip

RUN pip3 install yq

RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/${TARGETOS}/${TARGETARCH}/kubectl && chmod +x ./kubectl && mv ./kubectl /usr/local/bin/kubectl

ENV GOPATH=/home/ubuntu/go/
RUN curl -L "https://github.com/controlplaneio/kubesec/releases/download/v2.11.5/kubesec_${TARGETOS}_${TARGETARCH}.tar.gz" > kubesec.tar.gz && tar -xvf kubesec.tar.gz && mv kubesec /usr/bin/ && rm kubesec.tar.gz

COPY main.sh .

USER 1000
CMD ["main.sh"]

