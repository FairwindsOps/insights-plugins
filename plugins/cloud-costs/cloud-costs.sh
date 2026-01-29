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
  --provider <cloud provider - aws, gcp, or azure>
  --tagprefix <tag prefix - optional for aws, not used for GCP/Azure> \
  --tagkey <tag key - required for AWS, optional for GCP/Azure> \
  --tagvalue <tag value - required for AWS, GCP, and Azure> \
  --database <database name - required for AWS> \
  --table <table name for - required for AWS, optional for GCP if projectname, dataset and billingaccount are provided> \
  --catalog <catalog for - required for AWS> \
  --workgroup <workgroup - required for AWS> \
  --projectname <project name - required for GCP> \
  --dataset <dataset name - required for GCP if table is not provided> \
  --billingaccount <billing account - required for GCP if table is not provided> \
  --subscription <subscription ID - required for Azure> \
  [--format <data format - 'standard' or 'focus' (AWS/GCP only, Azure always uses FOCUS)>] \
  [--focusview <FOCUS view name - required for GCP when format is focus>] \
  [--timeout <time in seconds>] \
  [--days <number of days to query, default is 5>]

This script runs cloud costs integration for Fairwinds Insights.
EOF
}

provider=''
tagprefix=''
tagkey=''
tagvalue=''
database=''
table=''
timeout='60'
workgroup=''
projectname=''
dataset=''
billingaccount=''
subscription=''
days=''
format=''
focusview=''
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
        subscription)
            subscription=${2}
            ;;
        format)
            format=${2}
            ;;
        focusview)
            focusview=${2}
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
# Azure uses YYYY-MM-DD format
initial_date=$(date -u -d  $days+' day ago' +"%Y-%m-%d")
final_date=$(date -u +"%Y-%m-%d")

if  [[ "$provider" = "aws" ]]; then
  echo "AWS CUR Integration......"
  if [[ "$tagkey" = "" || "$tagvalue" = "" || "$database" = "" || "$table" = "" || "$catalog" = "" || "$workgroup" = "" || "$days" = "" ]]; then
    usage
    exit 1
  fi


  # Tag filter:
  # - If tagprefix is provided, use legacy CUR flattened columns (e.g. resource_tags_user_kubernetes_cluster)
  # - Otherwise, assume a "tags" MAP column is available (common in newer Athena exports)
  if [[ "$tagprefix" != "" ]]; then
    tag_filter="$tagprefix$tagkey='$tagvalue'"
  else
    # tags is a MAP(varchar,varchar), so use element_at() or bracket notation
    # try() handles cases where tags might be null or the key doesn't exist
    tag_filter="try(coalesce(element_at(tags, '$tagkey'), tags['$tagkey'])) = '$tagvalue'"
  fi

  if [[ "$format" == "focus" ]]; then
    echo "Using FOCUS format..."
    # The athena_data_exports_database.data table is already in FOCUS shape.
    # So we MUST query FOCUS columns (e.g. ChargePeriodEnd), not CUR columns (e.g. line_item_usage_end_date).
    #
    # To avoid guessing exact column names/casing, we DESCRIBE the table and map to known FOCUS fields.

    # 1) Query information_schema.columns to get column names/types
    # (Athena can be picky about quoting in DESCRIBE, but information_schema is stable.)
    describeQueryResults=$(aws athena start-query-execution \
      --query-string "SELECT column_name, data_type FROM information_schema.columns WHERE table_schema = '$database' AND table_name = '$table' ORDER BY ordinal_position" \
      --work-group "$workgroup" \
      --query-execution-context Database=$database,Catalog=$catalog \
)
    describeExecutionId=$(echo "$describeQueryResults" | jq .QueryExecutionId | sed 's/\"//g')

    describeAttempts=0
    while [ $describeAttempts -lt $timeout ]
    do
      echo "Athena DESCRIBE is running......"
      describeAttempts=$(( $describeAttempts + 1 ))
      sleep 1s;

      describeStatus=$(aws athena get-query-execution --query-execution-id $describeExecutionId --query 'QueryExecution.Status.State' --output text);
      echo "Athena DESCRIBE status=$describeStatus";

      if [ $describeStatus != "RUNNING" ]; then
        echo "Athena DESCRIBE Finished"
        if [ $describeStatus != "SUCCEEDED" ]; then
          echo "Athena DESCRIBE failed with status=$describeStatus"
          aws athena get-query-execution --query-execution-id $describeExecutionId --output json
          exit 3
        fi
        break
      fi
    done

    aws athena get-query-results --query-execution-id $describeExecutionId > "$OUTPUT_DIR/aws-describe.json"

    sql="SELECT
      \"servicename\" AS service_name,
      \"servicecategory\" AS service_category,
      \"chargesubcategory\" AS charge_sub_category,
      \"chargecategory\" AS charge_category,
      \"regionid\" AS region_id,
      \"resourceid\" AS resource_id,
      date_format(cast(\"chargeperiodstart\" as timestamp), '%Y-%m-%dT%H:%i:%sZ') AS charge_period_start,
      date_format(cast(\"chargeperiodend\" as timestamp), '%Y-%m-%dT%H:%i:%sZ') AS charge_period_end,
      SUM(cast(\"billedcost\" as double)) AS billed_cost,
      SUM(cast(\"effectivecost\" as double)) AS effective_cost,
      SUM(cast(\"consumedquantity\" as double)) AS consumed_quantity,
      \"consumedunit\" AS consumed_unit,
      \"billingcurrency\" AS billing_currency,
      \"billingaccountid\" AS billing_account_id,
      \"subaccountid\" AS sub_account_id,
      \"x_instancetype\" AS x_instance_type,
      \"x_vcpu\" AS x_vcpu,
      \"x_memory\" AS x_memory,
      \"x_gpu\" AS x_gpu,
      arbitrary(\"chargedescription\") AS charge_description
    FROM
      \"$database\".\"$table\"
    WHERE
      $tag_filter
      AND cast(\"chargeperiodend\" as timestamp) > timestamp '$initial_date_time'
      AND cast(\"chargeperiodend\" as timestamp) <= timestamp '$final_date_time'
    GROUP BY 1,2,3,4,5,6,7,8,12,13,14,15,16,17,18,19
    ORDER BY charge_period_start, service_name"

    queryResults=$(aws athena start-query-execution \
      --query-string "$sql" \
      --work-group "$workgroup" \
      --query-execution-context Database=$database,Catalog=$catalog \
)
  else
    # Standard query (original format)
    queryResults=$(aws athena start-query-execution \
    --query-string \
        "SELECT \
          line_item_product_code, identity_time_interval, sum(line_item_unblended_cost) AS cost, \
          line_item_usage_type, product_memory, product_instance_type, product_vcpu, product_clock_speed, sum(1) AS count, \
          sum(line_item_usage_amount) AS line_item_usage_amount, line_item_operation, product_product_family, product_gpu \
        FROM \
          "$database"."$table" \
        WHERE \
          $tag_filter \
          AND line_item_usage_end_date > timestamp '$initial_date_time' \
          AND line_item_usage_end_date <= timestamp '$final_date_time' \
        GROUP BY  1,2,4,5,6,7,8,11,12,13
        Order by 1, 2" \
    --work-group "$workgroup" \
    --query-execution-context Database=$database,Catalog=$catalog \
)
  fi

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

  aws athena get-query-results --query-execution-id $executionId > "$OUTPUT_DIR/cloudcosts-tmp.json"

  if [[ "$format" == "focus" ]]; then
    echo "Transforming to FOCUS format..."
    # Transform Athena results to FOCUS-compliant JSON array
    # Write directly to final filename to avoid mv issues
    jq '[
      .ResultSet.Rows[1:][] |
      .Data |
      {
        ServiceName: .[0].VarCharValue,
        ServiceCategory: .[1].VarCharValue,
        ChargeSubCategory: .[2].VarCharValue,
        ChargeCategory: (.[3].VarCharValue // "Usage"),
        RegionId: .[4].VarCharValue,
        ResourceId: .[5].VarCharValue,
        ChargePeriodStart: .[6].VarCharValue,
        ChargePeriodEnd: .[7].VarCharValue,
        BilledCost: (.[8].VarCharValue | if . then tonumber else 0 end),
        EffectiveCost: (.[9].VarCharValue | if . then tonumber else 0 end),
        ConsumedQuantity: (.[10].VarCharValue | if . then tonumber else 0 end),
        ConsumedUnit: .[11].VarCharValue,
        BillingCurrency: .[12].VarCharValue,
        BillingAccountId: .[13].VarCharValue,
        SubAccountId: .[14].VarCharValue,
        x_InstanceType: .[15].VarCharValue,
        x_vCPU: .[16].VarCharValue,
        x_Memory: .[17].VarCharValue,
        x_GPU: .[18].VarCharValue,
        ChargeDescription: .[19].VarCharValue,
        ProviderName: "Amazon Web Services"
      }
    ]' "$OUTPUT_DIR/cloudcosts-tmp.json" > "$OUTPUT_DIR/cloudcosts.json"

    if [[ $? -ne 0 ]]; then
      echo "Error transforming to FOCUS format"
      cat "$OUTPUT_DIR/cloudcosts-tmp.json"
      exit 1
    fi

    if [[ ! -f "$OUTPUT_DIR/cloudcosts.json" ]]; then
      echo "Error: Output file was not created"
      exit 1
    fi

    # Debug: Show file was created
    file_size=$(stat -f%z "$OUTPUT_DIR/cloudcosts.json" 2>/dev/null || stat -c%s "$OUTPUT_DIR/cloudcosts.json" 2>/dev/null || echo "unknown")
    echo "File created: $OUTPUT_DIR/cloudcosts.json (size: $file_size bytes)"
    ls -lh "$OUTPUT_DIR/cloudcosts.json" || true

    rm -f "$OUTPUT_DIR/cloudcosts-tmp.json"
    echo "Saved AWS costs file in FOCUS format to $OUTPUT_DIR/cloudcosts.json"
  else
    mv "$OUTPUT_DIR/cloudcosts-tmp.json" "$OUTPUT_DIR/cloudcosts.json"
    echo "Saved AWS costs file in $OUTPUT_DIR/cloudcosts.json"
  fi
  exit 0
fi
if [[ "$provider" == "gcp" ]]; then
  echo "Google Cloud integration......"

  if [[ "$tagvalue" = "" ]]; then
    usage
    exit 1
  fi
  if [[ "$projectname" = "" ]]; then
    usage
    exit 1
  fi
  
  if [[ "$tagkey" = "" ]]; then
    tagkey="goog-k8s-cluster-name"
  fi
  if [[ "$table" = "" ]]; then
    if [[ "$projectname" = "" || "$dataset" = "" || "$billingaccount" = "" ]]; then
      usage
      exit 1
    fi
    billingaccount=${billingaccount//-/_}
    table="$projectname.$dataset.gcp_billing_export_resource_v1_$billingaccount"
  fi

  echo "Google BigQuery is running......"

  if [[ "$format" == "focus" ]]; then
    if [[ "$focusview" == "" ]]; then
      echo "Error: --focusview is required when using FOCUS format"
      usage
      exit 1
    fi
    echo "Using FOCUS format from view: $focusview"
    sql="SELECT * FROM \`$focusview\` WHERE ChargePeriodStart >= '$initial_date_time' AND ChargePeriodStart < '$final_date_time' AND EXISTS (SELECT 1 FROM UNNEST(Tags) AS t WHERE t.key = '$tagkey' AND t.value = '$tagvalue') ORDER BY ChargePeriodStart DESC"
  else
    sql="SELECT cost, 0.0 AS cost_at_list, '' AS cost_type, service, sku, usage, usage_start_time, usage_end_time, CASE WHEN machine_spec IS NOT NULL OR accelerator_type IS NOT NULL THEN [ STRUCT('compute.googleapis.com/machine_spec' AS key, machine_spec AS value), STRUCT('compute.googleapis.com/cores' AS key, total_cores AS value), STRUCT('compute.googleapis.com/memory' AS key, total_memory AS value), STRUCT('compute.googleapis.com/accelerator_type' AS key, accelerator_type AS value), STRUCT('compute.googleapis.com/accelerator_count' AS key, accelerator_count AS value) ] ELSE NULL END AS system_labels, resource_type FROM ( SELECT SUM(main.cost) AS cost, STRUCT(main.service.description) AS service, STRUCT(main.sku.description AS description) AS sku, main.usage_start_time, main.usage_end_time, STRUCT(SUM(main.usage.amount) AS amount, SUM(main.usage.amount_in_pricing_units) AS amount_in_pricing_units, '' AS pricing_unit, '' AS unit) AS usage, (SELECT value FROM UNNEST(system_labels) WHERE key = 'compute.googleapis.com/machine_spec') AS machine_spec, (SELECT value FROM UNNEST(system_labels) WHERE key = 'compute.googleapis.com/cores') AS total_cores, (SELECT value FROM UNNEST(system_labels) WHERE key = 'compute.googleapis.com/memory') AS total_memory, (SELECT value FROM UNNEST(system_labels) WHERE key = 'compute.googleapis.com/accelerator_type') AS accelerator_type, (SELECT value FROM UNNEST(system_labels) WHERE key = 'compute.googleapis.com/accelerator_count') AS accelerator_count, CASE WHEN LOWER(main.sku.description) LIKE '%gpu%' OR LOWER(main.sku.description) LIKE '%nvidia%' OR LOWER(main.sku.description) LIKE '%tesla%' OR LOWER(main.sku.description) LIKE '%tpu%' OR LOWER(main.sku.description) LIKE '%accelerator%' THEN 'GPU' ELSE 'Compute' END AS resource_type FROM \`$table\` AS main LEFT JOIN UNNEST(labels) AS labels WHERE labels.key = '$tagkey' AND labels.value = '$tagvalue' AND usage_start_time >= '$initial_date_time' AND usage_start_time < '$final_date_time' AND TIMESTAMP_TRUNC(_PARTITIONTIME, DAY) >= '$initial_date_time' AND TIMESTAMP_TRUNC(_PARTITIONTIME, DAY) <= '$final_date_time' GROUP BY main.service.description, usage_start_time, usage_end_time, main.sku.description, machine_spec, total_memory, total_cores, accelerator_type, accelerator_count) ORDER BY usage_start_time DESC"
  fi

  /google-cloud-sdk/bin/bq --format=prettyjson --project_id $projectname  query --max_rows=10000000 --nouse_legacy_sql "$sql" > /output/cloudcosts-tmp.json  && echo "Executing..."
  if grep -q "BigQuery error" /output/cloudcosts-tmp.json; then
    echo "Error executing the query"
    cat /output/cloudcosts-tmp.json
    exit 1
  fi

  if ! grep -q "\[" /output/cloudcosts-tmp.json; then
    echo "No data found for the given tag value"
    cat /output/cloudcosts-tmp.json
    exit 1
  fi

  echo "Google BigQuey finished..."
  awk '$0 == "[" {p=1} p' < /output/cloudcosts-tmp.json > /output/cloudcosts-tmp-clean.json

  mv /output/cloudcosts-tmp-clean.json /output/cloudcosts.json
  echo "Saved GCP costs file in /output/cloudcosts.json"

  exit 0
fi
if [[ "$provider" == "azure" ]]; then
  echo "Azure Cost Management integration (FOCUS format)......"

  if [[ "$tagvalue" = "" ]]; then
    echo "Error: --tagvalue is required for Azure"
    usage
    exit 1
  fi
  if [[ "$subscription" = "" ]]; then
    echo "Error: --subscription is required for Azure"
    usage
    exit 1
  fi

  if [[ "$tagkey" = "" ]]; then
    tagkey="kubernetes-cluster"
  fi

  # Azure cost data takes 24-72 hours to finalize
  # Apply 2-day lag to ensure complete data (ignore today and yesterday)
  lag_days=2
  initial_date=$(date -u -d "$((days + lag_days)) day ago" +"%Y-%m-%d")
  final_date=$(date -u -d "$lag_days day ago" +"%Y-%m-%d")

  echo "Azure Cost Management query is running......"
  echo "Querying costs from $initial_date to $final_date for tag $tagkey=$tagvalue"
  echo "(2-day lag applied to ensure cost data is finalized)"

  # FOCUS format request body
  request_body=$(cat <<EOF
{
  "type": "ActualCost",
  "timeframe": "Custom",
  "timePeriod": {
    "from": "$initial_date",
    "to": "$final_date"
  },
  "dataset": {
    "granularity": "Daily",
    "aggregation": {
      "totalCost": {
        "name": "Cost",
        "function": "Sum"
      },
      "totalQuantity": {
        "name": "UsageQuantity",
        "function": "Sum"
      }
    },
    "grouping": [
      {"type": "Dimension", "name": "ServiceName"},
      {"type": "Dimension", "name": "MeterCategory"},
      {"type": "Dimension", "name": "MeterSubCategory"},
      {"type": "Dimension", "name": "ResourceLocation"},
      {"type": "Dimension", "name": "ResourceId"},
      {"type": "Dimension", "name": "ResourceGroupName"},
      {"type": "Dimension", "name": "ChargeType"},
      {"type": "Dimension", "name": "PublisherType"}
    ],
    "filter": {
      "tags": {
        "name": "$tagkey",
        "operator": "In",
        "values": ["$tagvalue"]
      }
    }
  }
}
EOF
)

  # Query Azure Cost Management API
  az rest --method post \
    --url "https://management.azure.com/subscriptions/$subscription/providers/Microsoft.CostManagement/query?api-version=2023-11-01" \
    --body "$request_body" \
    --output json > /output/cloudcosts-tmp.json

  if [[ $? -ne 0 ]]; then
    echo "Error executing Azure Cost Management query"
    cat /output/cloudcosts-tmp.json
    exit 1
  fi

  # Check for errors in response
  if grep -q '"error"' /output/cloudcosts-tmp.json; then
    echo "Azure Cost Management API returned an error"
    cat /output/cloudcosts-tmp.json
    exit 1
  fi

  echo "Transforming to FOCUS format..."
  # Transform Azure response to FOCUS-compliant format
  jq '[.properties.rows[] | {
    BilledCost: .[0],
    EffectiveCost: .[0],
    ConsumedQuantity: .[1],
    ChargePeriodStart: (.[2] | tostring | "\(.[0:4])-\(.[4:6])-\(.[6:8])T00:00:00Z"),
    ChargePeriodEnd: (.[2] | tostring | "\(.[0:4])-\(.[4:6])-\(.[6:8])T23:59:59Z"),
    ServiceName: .[3],
    ServiceCategory: .[4],
    ChargeSubCategory: .[5],
    RegionId: .[6],
    ResourceId: .[7],
    x_ResourceGroupName: .[8],
    ChargeCategory: (.[9] // "Usage"),
    PublisherName: (if .[10] == "Azure" then "Microsoft Azure" else .[10] end),
    BillingCurrency: .[11],
    ProviderName: "Microsoft Azure",
    BillingAccountId: "'"$subscription"'"
  }]' /output/cloudcosts-tmp.json > /output/cloudcosts-focus.json

  if [[ $? -ne 0 ]]; then
    echo "Error transforming to FOCUS format"
    cat /output/cloudcosts-tmp.json
    exit 1
  fi

  mv /output/cloudcosts-focus.json /output/cloudcosts.json
  rm -f /output/cloudcosts-tmp.json
  echo "Azure Cost Management query finished..."
  echo "Saved Azure costs file in FOCUS format to /output/cloudcosts.json"

  exit 0
fi
echo "--provider is required and should be aws, gcp, or azure"
exit 1