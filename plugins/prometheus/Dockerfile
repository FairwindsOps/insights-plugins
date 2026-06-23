FROM alpine:3.24.1

RUN apk --no-cache add ca-certificates \
    && apk --no-cache add --upgrade libcrypto3 libssl3 zlib
COPY prometheus-collector /usr/local/bin/

CMD ["prometheus-collector"]
