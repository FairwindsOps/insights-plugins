FROM amazon/aws-cli:2.2.29

RUN yum install jq -y

COPY aws-costs.sh aws-costs.sh

CMD ["aws-costs.sh"]