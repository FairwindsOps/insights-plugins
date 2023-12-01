FROM gcr.io/google.com/cloudsdktool/google-cloud-cli:455.0.0

RUN apt-get update && \
    apt-get install -y \
        python3 \
        python3-pip \
        python3-setuptools \
        groff \
        less \
        jq \
    && pip3 install --upgrade pip \
    && apt-get clean

RUN pip3 --no-cache-dir install --upgrade awscli

COPY cloud-costs.sh cloud-costs.sh

CMD ["cloud-costs.sh"]
