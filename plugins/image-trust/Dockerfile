FROM alpine:3.23.4
WORKDIR /usr/local/bin
RUN apk --no-cache add ca-certificates \
    && apk --no-cache add --upgrade libcrypto3 libssl3 zlib

RUN addgroup -S insights && adduser -u 1200 -S insights -G insights
USER 1200
COPY image-trust .
COPY report.sh /opt/app/report.sh

WORKDIR /opt/app

CMD ["sh", "./report.sh"]
