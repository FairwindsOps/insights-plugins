#! /bin/bash
#set -e
#set -x

if [ "$DEBUG" != "" ]
then
    set +x
fi
usage()
{
cat << EOF
usage: awscosts --cluster <cluster name>

This script runs aws costs for Fairwinds Insights.
EOF
}

cluster=''

while [ ! $# -eq 0 ]; do
    flag=${1##-}
    flag=${flag##-}
    value=${2}
    case "$flag" in
        c | cluster)
            cluster=${2}
            ;;
        *)
            usage
            exit
            ;;
    esac
    shift
    shift
done
if [ "$cluster" = "" ]; then
  usage
  exit 1
fi

month_str=$(date +%m)
month=$(expr $month_str + 0)
echo "$cluster"
queryResults=$(aws athena start-query-execution \
--query-string \
    "SELECT line_item_product_code, sum(line_item_blended_cost) AS cost \
    FROM "athena_cur_database"."fairwinds_insights_cur_report" \
    WHERE \
    resource_tags_user_kubernetes_cluster='$cluster' \
    AND year='$(date +'%Y')' \
    AND year='$(date +'%Y')' \
    AND month='$month' \
    GROUP BY  1 \
    ORDER BY  1" \
--work-group "cur_athena_workgroup" \
--query-execution-context Database=athena_cur_database,Catalog=AwsDataCatalog)

executionId=$(echo $queryResults | jq .QueryExecutionId | sed 's/"//g')

for i in 1 2 3 4 5 6 7 8 9 10; do
  echo "Athena query is running......"
  sleep 1s;
  
  status=$(aws athena get-query-execution --query-execution-id $executionId --query 'QueryExecution.Status.State' --output text);
  echo "Athena query status=$status";
  
  if [ $status != "RUNNING" ]; then
    echo "Athena query Finished"
    if [ $status != "SUCCEEDED" ]; then
    echo "Athena failed to execute query with status=$status"
    exit 3
  fi
  break
fi
done

aws athena get-query-results --query-execution-id $executionId > /output/aws-costs-tmp.json
mv /output/aws-costs-tmp.json /output/aws-costs.json

echo "Saved aws costs file in /output/aws-costs.json"