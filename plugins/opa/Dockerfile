FROM alpine:3.23
WORKDIR /usr/local/bin
RUN apk -U upgrade
RUN apk --no-cache add ca-certificates

USER 1200
COPY fw-opa .

WORKDIR /opt/app

CMD ["fw-opa"]
