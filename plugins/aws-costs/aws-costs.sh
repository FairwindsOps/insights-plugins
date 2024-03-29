#! /bin/bash
set -e
set -x

if [ "$DEBUG" != "" ]
then
    set +x
fi
usage()
{
cat << EOF
usage: awscosts \
  --database <database name> \
  --table <table name> \
  --tagprefix <tag prefix> \
  --tagkey <tag key> \
  --tagvalue <tag value> \
  --catalog <catalog> \
  --workgroup <workgroup> \
  [--timeout <time in seconds>] \
  [--days <number of days to query, default is 5>]

This script runs aws costs for Fairwinds Insights.
EOF
}

tagkey=''
tagprefix=''
tagvalue=''
database=''
table=''
timeout='60'
workgroup=''
days=''
while [ ! $# -eq 0 ]; do
    flag=${1##-}
    flag=${flag##-}
    value=${2}
    case "$flag" in
        timeout)
            timeout=${2}
            ;;            
        tagprefix)
            tagprefix=${2}
            ;;
        tagkey)
            tagkey=${2}
            ;;
        tagvalue)
            tagvalue=${2}
            ;;
        database)
            database=${2}
            ;;
        table)
            table=${2}
            ;;
        catalog)
            catalog=${2}
            ;;
        days)
            days=${2}
            ;;
        workgroup)
            workgroup=${2}
            ;;
        *)
            usage
            exit
            ;;
    esac
    shift
    shift
done
if [[ "$days" = "" && "$AWS_COSTS_DAYS" != "" ]]; then
  days=$AWS_COSTS_DAYS
fi
if [[ "$days" = "" ]]; then
  days='5'
fi
if [[ "$tagkey" = "" || "$tagvalue" = "" || "$database" = "" || "$table" = "" || "$catalog" = "" || "$workgroup" = "" || "$days" = "" ]]; then
  usage
  exit 1
fi

initial_date_time=$(date -u -d  $days+' day ago' +"%Y-%m-%d %H:00:00.000")
final_date_time=$(date -u +"%Y-%m-%d %H:00:00.000")

queryResults=$(aws athena start-query-execution \
--query-string \
    "SELECT \
      line_item_product_code, identity_time_interval, sum("line_item_unblended_cost") AS cost, \
      line_item_usage_type, product_memory, product_instance_type, product_vcpu, product_clock_speed, sum(1) AS count, \
      sum(line_item_usage_amount) AS line_item_usage_amount, line_item_operation, product_product_family \
    FROM \
      "$database"."$table" \
    WHERE \
      $tagprefix$tagkey='$tagvalue' \
      AND line_item_usage_end_date > timestamp '$initial_date_time' \
      AND line_item_usage_end_date <= timestamp '$final_date_time' \
    GROUP BY  1,2,4,5,6,7,8,11,12
    Order by 1, 2" \
--work-group "$workgroup" \
--query-execution-context Database=$database,Catalog=$catalog)

executionId=$(echo $queryResults | jq .QueryExecutionId | sed 's/"//g')

attempts=0
while [ $attempts -lt $timeout ]
do
  echo "Athena query is running......"
  attempts=$(( $attempts + 1 ))
  sleep 1s;

  status=$(aws athena get-query-execution --query-execution-id $executionId --query 'QueryExecution.Status.State' --output text);
  echo "Athena query status=$status";
  
  if [ $status != "RUNNING" ]; then
    echo "Athena query Finished"
    if [ $status != "SUCCEEDED" ]; then
    echo "Athena failed to execute query with status=$status"
    echo "Displaying full query-execution output:"
    aws athena get-query-execution --query-execution-id $executionId --output json
    exit 3
  fi
  break
fi
done

aws athena get-query-results --query-execution-id $executionId > /output/awscosts-tmp.json
mv /output/awscosts-tmp.json /output/awscosts.json

echo "Saved aws costs file in /output/awscosts.json"
