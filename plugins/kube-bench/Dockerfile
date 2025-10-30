FROM aquasec/kube-bench:v0.13.0

ARG TARGETARCH
ARG TARGETOS
RUN apk -U upgrade
RUN apk --no-cache add bash ca-certificates curl

RUN rm /usr/local/bin/kubectl
ENV kubectlVersion=1.33.4
RUN curl -LO https://dl.k8s.io/release/$kubectlVersion/bin/${TARGETOS}/${TARGETARCH}/kubectl
RUN chmod +x ./kubectl && mv ./kubectl /usr/local/bin/kubectl

COPY kube-bench-daemon /usr/local/bin/

ENTRYPOINT []
CMD ["kube-bench-daemon"]
