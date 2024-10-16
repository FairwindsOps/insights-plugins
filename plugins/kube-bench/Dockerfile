FROM aquasec/kube-bench:v0.9.1

RUN apk -U upgrade
RUN apk --no-cache add bash ca-certificates curl

COPY kube-bench-daemon /usr/local/bin/

ENTRYPOINT []
CMD ["kube-bench-daemon"]
