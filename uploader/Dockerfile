FROM alpine:3.12.0
WORKDIR /usr/local/bin
RUN apk --no-cache add ca-certificates curl
COPY . .
ENTRYPOINT ["uploader.sh"]
