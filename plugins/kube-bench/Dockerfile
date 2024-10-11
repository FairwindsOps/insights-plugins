FROM aquasec/kube-bench:v0.9.0
RUN apk --no-cache add ca-certificates
RUN apk -U upgrade
RUN apk add curl
ENV kubectlVersion=1.31.0
RUN curl -LO https://dl.k8s.io/release/$kubectlVersion/bin/${TARGETOS}/${TARGETARCH}/kubectl
RUN chmod +x ./kubectl && mv ./kubectl /usr/local/bin/kubectl

COPY kube-bench-daemon /usr/local/bin/

ENTRYPOINT []
CMD ["kube-bench-daemon"]
