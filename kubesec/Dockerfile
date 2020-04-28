FROM ubuntu:20.04

ENV DEBIAN_FRONTEND=noninteractive

# Is curl only used for build? Can we move to a multistage? Can we remove some of these other packages?
RUN apt-get update && apt-get install -y golang-go curl git jq python3 python3-pip tar && apt-get clean

RUN pip3 install yq

RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl && chmod +x ./kubectl && mv ./kubectl /usr/local/bin/kubectl

ENV GOPATH=/home/ubuntu/go/
RUN curl -L "https://github.com/controlplaneio/kubesec/releases/download/v2.2.0/kubesec_linux_amd64.tar.gz" > kubesec.tar.gz && tar -xvf kubesec.tar.gz && mv kubesec /usr/bin/ && rm kubesec.tar.gz

COPY . .

USER 1000
CMD ["main.sh"]

