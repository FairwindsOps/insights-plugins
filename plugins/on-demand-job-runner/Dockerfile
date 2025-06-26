FROM alpine:3.22
WORKDIR /usr/local/bin
RUN apk -U upgrade
RUN apk --no-cache add ca-certificates

USER 1200
COPY on-demand-job-runner .

WORKDIR /opt/app

CMD ["on-demand-job-runner"]
