FROM aquasec/kube-bench:v0.8.0
RUN apk --no-cache add ca-certificates
RUN apk -U upgrade

COPY kube-bench-daemon /usr/local/bin/

ENTRYPOINT []
CMD ["kube-bench-daemon"]
