FROM amazon/aws-cli:2.7.25

RUN yum install jq -y
RUN yum update -y

COPY aws-costs.sh aws-costs.sh

CMD ["aws-costs.sh"]
