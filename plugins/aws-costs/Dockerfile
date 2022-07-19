FROM amazon/aws-cli:2.7.9

RUN yum install jq -y

COPY aws-costs.sh aws-costs.sh

CMD ["aws-costs.sh"]
