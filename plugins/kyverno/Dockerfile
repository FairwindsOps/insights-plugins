FROM alpine:3.23.4
WORKDIR /insights
RUN apk --no-cache add ca-certificates \
    && apk --no-cache add --upgrade libcrypto3 libssl3 zlib
COPY kyverno /usr/local/bin/insights-kyverno

CMD ["insights-kyverno"]
