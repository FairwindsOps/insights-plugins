FROM alpine:3.11.5
WORKDIR /usr/local/bin
RUN apk --no-cache add ca-certificates curl
COPY . .
ENTRYPOINT ["uploader.sh"]
