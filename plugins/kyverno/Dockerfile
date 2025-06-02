FROM alpine:3.22
WORKDIR /insights
RUN apk -U upgrade
COPY kyverno /usr/local/bin/insights-kyverno

CMD ["insights-kyverno"]
