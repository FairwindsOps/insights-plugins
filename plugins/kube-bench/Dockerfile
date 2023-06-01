FROM aquasec/kube-bench:v0.6.14
RUN apk --no-cache add ca-certificates
RUN apk -U upgrade

COPY kube-bench-daemon /usr/local/bin/

ENTRYPOINT []
CMD ["kube-bench-daemon"]
