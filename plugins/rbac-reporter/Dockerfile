FROM alpine:3.20
WORKDIR /usr/local/bin
RUN apk -U upgrade
RUN apk --no-cache add ca-certificates

RUN addgroup -S insights && adduser -u 1200 -S insights -G insights
USER 1200
COPY rbac-reporter .

WORKDIR /opt/app

CMD ["rbac-reporter"]
