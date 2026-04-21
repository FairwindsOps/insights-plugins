FROM alpine:3.23.4
WORKDIR /usr/local/bin
RUN apk --no-cache add ca-certificates \
    && apk --no-cache add --upgrade libcrypto3 libssl3 zlib

USER 1200
COPY fw-opa .

WORKDIR /opt/app

CMD ["fw-opa"]
