FROM amazon/aws-cli:2.11.4

RUN yum install jq -y
RUN yum update -y

COPY cloud-costs.sh cloud-costs.sh

CMD ["cloud-costs.sh"]
