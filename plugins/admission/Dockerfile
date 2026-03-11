FROM alpine:3.23
WORKDIR /insights
RUN apk update && apk -U upgrade
COPY insights-admission /usr/local/bin/insights-admission

CMD ["insights-admission"]
