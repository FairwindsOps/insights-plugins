FROM alpine:3.18
WORKDIR /insights
RUN apk -U upgrade
COPY insights-admission /usr/local/bin/insights-admission

CMD ["insights-admission"]
