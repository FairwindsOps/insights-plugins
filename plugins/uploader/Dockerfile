FROM alpine:3.15
WORKDIR /usr/local/bin
RUN echo "TODO remove this"
RUN apk --no-cache add ca-certificates curl

RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl && chmod +x ./kubectl && mv ./kubectl /usr/local/bin/kubectl

COPY . .
ENTRYPOINT ["uploader.sh"]
