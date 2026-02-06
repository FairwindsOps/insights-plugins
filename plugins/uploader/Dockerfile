FROM alpine:3.23
ARG TARGETARCH
ARG TARGETOS
WORKDIR /usr/local/bin
RUN apk -U upgrade
RUN apk --no-cache add bash ca-certificates curl

# curl -L -s https://dl.k8s.io/release/stable.txt
ARG kubectlVersion=1.35.0
# KUBECTL_BASE_URL override for air-gapped/proxy environments (e.g. https://internal-mirror.example.com)
ARG KUBECTL_BASE_URL=https://dl.k8s.io
# -f: fail on HTTP 4xx/5xx so we never save error pages as kubectl; verify ELF so bad responses still fail build
RUN curl -f -L -S -o kubectl "${KUBECTL_BASE_URL}/release/v${kubectlVersion}/bin/${TARGETOS}/${TARGETARCH}/kubectl" && \
    ( head -c 4 ./kubectl | od -A n -t x1 | grep -q 7f ) || ( echo "kubectl download failed or returned non-ELF (e.g. HTML/error page); first bytes:"; head -c 200 ./kubectl | od -A n -t x1; head -5 ./kubectl; exit 1 ) && \
    chmod +x ./kubectl && mv ./kubectl /usr/local/bin/kubectl

COPY download.sh .
COPY uploader.sh .
COPY version.txt .
ENTRYPOINT ["uploader.sh"]
