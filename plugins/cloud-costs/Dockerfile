FROM gcr.io/google.com/cloudsdktool/google-cloud-cli:473.0.0

RUN apt-get update && \
    apt-get install -y \
        groff \
        less \
        jq \
    && apt-get clean

RUN curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip" && \
    unzip awscliv2.zip && \
    ./aws/install && \
    rm -rf awscliv2.zip ./aws

COPY cloud-costs.sh cloud-costs.sh

CMD ["cloud-costs.sh"]
