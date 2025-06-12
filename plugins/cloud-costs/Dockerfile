FROM python:3-alpine3.22

ARG TARGETARCH
ARG TARGETOS

RUN apk -U upgrade
RUN apk update && apk upgrade
RUN apk add jq 
RUN apk add curl
RUN apk add bash
RUN apk add moreutils
RUN apk add coreutils
RUN apk add unzip


ENV google_cloud_sdk_version=502.0.0

RUN apk add --no-cache aws-cli

    

COPY cloud-costs.sh cloud-costs.sh

CMD ["cloud-costs.sh"]
