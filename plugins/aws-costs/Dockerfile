FROM amazon/aws-cli:2.2.29

RUN apk update && apk upgrade
RUN apk add jq

COPY aws-costs.sh aws-costs.sh

CMD ["aws-costs.sh"]