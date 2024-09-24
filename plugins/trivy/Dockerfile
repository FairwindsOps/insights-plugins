FROM alpine:3.20 AS downloader
ARG TARGETARCH
ARG TARGETOS
ENV trivyVersion=0.54.1
RUN apk update && apk add curl

ENV kubectlVersion=1.31.1
RUN curl -LO https://dl.k8s.io/release/$kubectlVersion/bin/${TARGETOS}/${TARGETARCH}/kubectl
RUN chmod +x ./kubectl && mv ./kubectl /usr/local/bin/kubectl

RUN if [ "${TARGETARCH}" = "amd64" ] ; then trivyArch="64bit"; else trivyArch="${TARGETARCH}"; fi && \
  curl -L https://github.com/aquasecurity/trivy/releases/download/v${trivyVersion}/trivy_${trivyVersion}_${TARGETOS}-${trivyArch}.tar.gz > trivy.tar.gz && tar -xvf trivy.tar.gz && mv ./trivy /usr/local/bin/trivy && rm trivy.tar.gz

FROM alpine:3.20
WORKDIR /trivy
RUN apk -U upgrade
COPY scan ./scan
COPY --from=downloader /usr/local/bin/kubectl /usr/local/bin/kubectl
COPY --from=downloader /usr/local/bin/trivy /usr/local/bin/trivy

RUN TRIVY_CACHE_DIR=/var/tmp trivy image --download-db-only

ENV HOME=/
RUN mkdir $HOME/.docker

RUN apk add --no-cache skopeo

COPY report.sh ./
CMD ./report.sh
