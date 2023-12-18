FROM alpine:3.19
WORKDIR /insights
RUN apk -U upgrade
COPY insights-admission /usr/local/bin/insights-admission

CMD ["insights-admission"]
