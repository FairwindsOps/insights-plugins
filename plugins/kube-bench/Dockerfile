FROM aquasec/kube-bench:v0.9.0
RUN apk --no-cache add ca-certificates
RUN apk -U upgrade
RUN apk add curl
ENV kubectlVersion=1.31.1
RUN curl -LO https://dl.k8s.io/release/$kubectlVersion/bin/${TARGETOS}/${TARGETARCH}/kubectl
RUN chmod +x ./kubectl && mv ./kubectl /usr/local/bin/kubectl

RUN apk add --no-cache --virtual .build-deps bash gcc musl-dev openssl go 

# download go tar 
RUN wget -O go.tgz https://dl.google.com/go/go1.22.7.src.tar.gz 
RUN tar -C /usr/local -xzf go.tgz 

COPY kube-bench-daemon /usr/local/bin/

ENTRYPOINT []
CMD ["kube-bench-daemon"]
