FROM alpine:3.21
WORKDIR /insights
RUN apk -U upgrade
COPY kyverno /usr/local/bin/insights-kyverno

CMD ["insights-kyverno"]
