FROM alpine:3.11.3
WORKDIR /usr/local/bin
RUN apk --no-cache add ca-certificates curl
COPY . .
ENTRYPOINT ["uploader.sh"]
