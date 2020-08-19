FROM alpine:3.12

RUN apk add curl
RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/v1.19.0-rc.4/bin/linux/amd64/kubectl && chmod +x ./kubectl && mv ./kubectl /usr/local/bin/kubectl

USER 10010
ENTRYPOINT [ "kubectl" ]
CMD [ "--help" ]
