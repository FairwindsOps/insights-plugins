FROM alpine:20230208

RUN apk -U upgrade
COPY prometheus-collector /usr/local/bin/

CMD ["prometheus-collector"]
