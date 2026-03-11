FROM alpine:3.23
WORKDIR /insights
RUN apk update && apk -U upgrade
COPY kyverno /usr/local/bin/insights-kyverno

CMD ["insights-kyverno"]
