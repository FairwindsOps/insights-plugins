FROM amazon/aws-cli:2.2.29

RUN yum install jq

COPY aws-costs.sh aws-costs.sh

CMD ["aws-costs.sh"]