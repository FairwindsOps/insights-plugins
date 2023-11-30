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
usage: cloud-costs \
  --provider <cloud provider - aws is default>
  --tagkey <tag key - required for AWS, optional for GCP> \
  --tagvalue <tag value - required for AWS and GCP> \
  --database <database name - required for AWS> \
  --table <table name for - required for AWS> \
  --catalog <catalog for - required for AWS> \
  --workgroup <workgroup - required for AWS> \
  --projectname <project name - required for GCP> \
  --dataset <dataset name - required for GCP> \
  --billingaccount <billing account - required for GCP> \
  [--timeout <time in seconds>] \
  [--days <number of days to query, default is 5>]

This script runs cloud costs integration for Fairwinds Insights.
EOF
}

provider=''
tagkey=''
tagvalue=''
database=''
table=''
timeout='60'
workgroup=''
projectname=''
dataset=''
billingaccount=''
days=''
while [ ! $# -eq 0 ]; do
    flag=${1##-}
    flag=${flag##-}
    value=${2}
    case "$flag" in
        timeout)
            timeout=${2}
            ;;            
        provider)
            provider=${2}
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
        workgroup)
            workgroup=${2}
            ;;
        projectname)
            projectname=${2}
            ;;
        dataset)
            dataset=${2}
            ;;
        billingaccount)
            billingaccount=${2}
            ;;
        *)
            usage
            exit
            ;;
    esac
    shift
    shift
done
if [[ "$days" = "" && "$CLOUD_COSTS_DAYS" != "" ]]; then
  days=$CLOUD_COSTS_DAYS
fi
if [[ "$days" = "" ]]; then
  days='5'
fi

initial_date_time=$(date -u -d  $days+' day ago' +"%Y-%m-%d %H:00:00.000")
final_date_time=$(date -u +"%Y-%m-%d %H:00:00.000")

if  [[ "$provider" = "aws" ]]; then
   echo "AWS CUR Integration......"
  if [[ "$tagkey" = "" || "$tagvalue" = "" || "$database" = "" || "$table" = "" || "$catalog" = "" || "$workgroup" = "" || "$days" = "" ]]; then
    usage
    exit 1
  fi

  queryResults=$(aws athena start-query-execution \
  --query-string \
      "SELECT \
        line_item_product_code, identity_time_interval, sum("line_item_unblended_cost") AS cost, \
        line_item_usage_type, product_memory, product_instance_type, product_vcpu, product_clock_speed, sum(1) AS count, \
        sum(line_item_usage_amount) AS line_item_usage_amount, line_item_operation, product_product_family \
      FROM \
        "$database"."$table" \
      WHERE \
        resource_tags_user_$tagkey='$tagvalue' \
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

  aws athena get-query-results --query-execution-id $executionId > /output/cloudcosts-tmp.json
  mv /output/cloudcosts-tmp.json /output/cloudcosts.json

  echo "Saved aws costs file in /output/cloudcosts.json"
  exit 0
fi
if [[ "$provider" == "gcp" ]]; then
  echo "Google Cloud integration......"

  if [[ "$tagvalue" = "" || "$projectname" = "" || "$dataset" = "" || "$billingaccount" = "" || "$days" = "" ]]; then
    usage
    exit 1
  fi
  
  if [[ "$tagkey" = "" ]]; then
    tagkey="goog-k8s-cluster-name"
  fi

  billingaccount=${billingaccount//-/_}
  table="$projectname.$dataset.gcp_billing_export_resource_v1_$billingaccount"

  echo "Google bigquey is running......"

  gcloud auth activate-service-account --key-file=/.config/gcp-key.json

  sql="SELECT main.* FROM \`$table\` AS main LEFT JOIN UNNEST(labels) as labels WHERE labels.key = '$tagkey' AND labels.value = '$tagvalue' and usage_start_time >= '$initial_date_time' AND usage_start_time < '$final_date_time' order by usage_start_time desc"
  bq --format=prettyjson query --max_rows=1000000 --nouse_legacy_sql "$sql" > /output/cloudcosts-tmp.json
  echo "Google bigquey finished......"
  mv /output/cloudcosts-tmp.json /output/cloudcosts.json

  echo "Saved GCP costs file in /output/cloudcosts.json"  
  exit 0
fi