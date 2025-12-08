FROM alpine:3.23
WORKDIR /usr/local/bin
RUN apk -U upgrade
RUN apk --no-cache add ca-certificates openssl

# Create directories for certificates
RUN mkdir -p /opt/certs

RUN addgroup -S insights && adduser -u 1200 -S insights -G insights
USER 1200
COPY insights-event-watcher .

WORKDIR /opt/app

CMD ["/usr/local/bin/insights-event-watcher"]
