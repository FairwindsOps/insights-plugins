FROM alpine:3.19

RUN apk -U upgrade
COPY prometheus-collector /usr/local/bin/

CMD ["prometheus-collector"]
