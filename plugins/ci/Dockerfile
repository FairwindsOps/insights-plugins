FROM alpine:3.20 AS downloader
ARG TARGETARCH
ARG TARGETOS
ENV tfsecVersion=1.28.6
ENV trivyVersion=0.50.4
ENV polarisVersion=9.0.0
ENV plutoVersion=5.19.4
ENV helmVersion=3.11.2

# Bash and openssl are  required by the Helm script.
RUN apk update && apk --no-cache add curl bash openssl ca-certificates
RUN if [ "${TARGETARCH}" = "amd64" ] ; then trivyArch="64bit"; else trivyArch="${TARGETARCH}"; fi && \
  curl -L https://github.com/aquasecurity/trivy/releases/download/v${trivyVersion}/trivy_${trivyVersion}_${TARGETOS}-${trivyArch}.tar.gz > trivy.tar.gz && tar -xvf trivy.tar.gz && mv ./trivy /usr/local/bin/trivy && rm trivy.tar.gz
RUN curl -L https://github.com/aquasecurity/tfsec/releases/download/v${tfsecVersion}/tfsec-${TARGETOS}-${TARGETARCH} > /usr/local/bin/tfsec && chmod +x /usr/local/bin/tfsec
RUN curl -L "https://github.com/FairwindsOps/polaris/releases/download/$polarisVersion/polaris_${TARGETOS}_${TARGETARCH}.tar.gz" > polaris.tar.gz && tar -xvf polaris.tar.gz && chmod +x polaris && rm polaris.tar.gz && mv ./polaris /usr/local/bin/polaris
RUN curl -L "https://github.com/FairwindsOps/pluto/releases/download/v$plutoVersion/pluto_${plutoVersion}_${TARGETOS}_${TARGETARCH}.tar.gz" > pluto.tar.gz && tar -xvf pluto.tar.gz && chmod +x pluto && rm pluto.tar.gz && mv ./pluto /usr/local/bin/pluto

RUN DESIRED_VERSION=$helmVersion curl https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3 | bash

FROM alpine:3.20
WORKDIR /insights
RUN apk -U upgrade
RUN apk add --no-cache skopeo libc6-compat
RUN apk add --no-cache git=2.43.4-r0

RUN mkdir -p /output/tmp
COPY insights-ci /usr/local/bin
COPY --from=downloader /usr/local/bin/trivy /usr/local/bin/trivy
COPY --from=downloader /usr/local/bin/tfsec /usr/local/bin/tfsec
COPY --from=downloader /usr/local/bin/polaris /usr/local/bin/polaris
COPY --from=downloader /usr/local/bin/helm /usr/local/bin/helm
COPY --from=downloader /usr/local/bin/pluto /usr/local/bin/pluto

CMD ["insights-ci"]
