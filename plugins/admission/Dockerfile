FROM alpine:3.24.0
WORKDIR /insights
RUN apk --no-cache add ca-certificates \
    && apk --no-cache add --upgrade libcrypto3 libssl3 zlib
COPY insights-admission /usr/local/bin/insights-admission

CMD ["insights-admission"]
