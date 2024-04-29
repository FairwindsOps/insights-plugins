FROM gcr.io/google.com/cloudsdktool/google-cloud-cli:473.0.0

RUN apt-get update -qqy && \
    apt-get -qqy install -y \
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
