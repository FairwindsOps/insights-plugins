FROM gcr.io/google.com/cloudsdktool/google-cloud-cli:455.0.0-alpine

RUN apk add --no-cache \
        python3 \
        py3-pip \
    && pip3 install --upgrade pip \
    && pip3 install --no-cache-dir \
        awscli \
    && rm -rf /var/cache/apk/*

COPY cloud-costs.sh cloud-costs.sh

CMD ["cloud-costs.sh"]
