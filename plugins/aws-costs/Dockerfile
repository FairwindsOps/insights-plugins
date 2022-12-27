FROM amazon/aws-cli:2.8.3

RUN yum install jq -y
RUN yum update -y

COPY aws-costs.sh aws-costs.sh

CMD ["aws-costs.sh"]
