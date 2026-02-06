FROM alpine:3.23
ARG TARGETARCH
ARG TARGETOS
WORKDIR /usr/local/bin
RUN apk -U upgrade
RUN apk --no-cache add bash ca-certificates curl

# curl -L -s https://dl.k8s.io/release/stable.txt
ENV kubectlVersion=1.35.0
# Verify we got a real ELF binary; failed downloads often write HTML error pages (causes "syntax error: unexpected redirection" at runtime)
RUN curl -LO https://dl.k8s.io/release/v${kubectlVersion}/bin/${TARGETOS}/${TARGETARCH}/kubectl && \
    ( head -c 4 ./kubectl | od -A n -t x1 | grep -q 7f ) || ( echo "kubectl download failed or returned non-ELF (e.g. HTML/error page); first bytes:"; head -c 200 ./kubectl | od -A n -t x1; head -5 ./kubectl; exit 1 ) && \
    chmod +x ./kubectl && mv ./kubectl /usr/local/bin/kubectl

COPY download.sh .
COPY uploader.sh .
COPY version.txt .
ENTRYPOINT ["uploader.sh"]
