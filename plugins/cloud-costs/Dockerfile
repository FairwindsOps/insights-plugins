FROM gcr.io/google.com/cloudsdktool/google-cloud-cli:473.0.0

RUN apt-get update && \
    apt-get install -y \
        groff \
        less \
        jq \
    && apt-get clean


COPY cloud-costs.sh cloud-costs.sh

CMD ["cloud-costs.sh"]
