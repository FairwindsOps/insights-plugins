FROM alpine:3.20
ARG TARGETARCH
ARG TARGETOS

RUN apk -U upgrade
RUN apk update && apk upgrade
RUN apk add jq bash curl moreutils

# curl -L -s https://dl.k8s.io/release/stable.txt
ENV kubectlVersion=v1.29.0
RUN curl -LO https://dl.k8s.io/release/$kubectlVersion/bin/${TARGETOS}/${TARGETARCH}/kubectl
RUN chmod +x ./kubectl && mv ./kubectl /usr/local/bin/kubectl

COPY main.sh .

USER 1000
CMD ["/main.sh"]
