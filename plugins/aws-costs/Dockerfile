FROM amazon/aws-cli:2.4.16

RUN yum install jq -y

COPY aws-costs.sh aws-costs.sh

CMD ["aws-costs.sh"]
