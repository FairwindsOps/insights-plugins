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
  --tagkey <tag key - required for AWS standard, optional for AWS FOCUS (uses aws:eks:cluster-name)> \
  --tagvalue <tag value - required for AWS, GCP, and Azure> \
  --database <database name - required for AWS> \
  --table <table name for - required for AWS, optional for GCP if projectname, dataset and billingaccount are provided> \
  --catalog <catalog for - required for AWS> \
  --workgroup <workgroup - required for AWS> \
  --projectname <project name - required for GCP, or derived from focusview when format is focus> \
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
            exit 1
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
OUTPUT_DIR=${OUTPUT_DIR:-.}

# Portable date: macOS uses -v-Nd, GNU date uses -d "N days ago"
if date -u -v-1d +"%Y-%m-%d" &>/dev/null; then
  date_ago() { date -u -v-${1}d +"$2"; }
else
  date_ago() { date -u -d "${1} days ago" +"$2"; }
fi

# Use full calendar days: start of day N days ago through end of today (so midnight timestamps are included)
initial_date_time=$(date_ago "$days" "%Y-%m-%d 00:00:00.000")
final_date_time=$(date -u +"%Y-%m-%d 23:59:59.999")
# Azure uses YYYY-MM-DD format
initial_date=$(date_ago "$days" "%Y-%m-%d")
final_date=$(date -u +"%Y-%m-%d")

if  [[ "$provider" = "aws" ]]; then
  echo "AWS CUR Integration......"
  if [[ "$tagvalue" = "" || "$database" = "" || "$table" = "" || "$catalog" = "" || "$workgroup" = "" || "$days" = "" ]]; then
    usage
    exit 1
  fi
  # For AWS standard format, tagkey is required; for FOCUS we use fixed key aws:eks:cluster-name
  if [[ "$format" != "focus" && "$tagkey" = "" ]]; then
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

    # AWS FOCUS data export table "data" uses lowercase column names (servicename, tags, chargeperiodend, etc.)
    # tags is map<string,string> in Athena; default tag name for EKS cluster is aws:eks:cluster-name (--tagkey optional)
    # Schema has no chargesubcategory, x_instancetype, x_vcpu, x_memory, x_gpu; use NULL for output shape
    # FOCUS date filter: use start of current year (matches DATE 'YYYY-01-01') so export lag doesn't exclude data
    focus_initial_date=$(date -u +"%Y-01-01")
    if [[ "$tagprefix" == "" ]]; then
      focus_tag_key="${tagkey:-aws:eks:cluster-name}"
      focus_tag_filter="element_at(tags, '$focus_tag_key') = '$tagvalue'"
    else
      focus_tag_filter="$tag_filter"
    fi
    sql="SELECT
      \"ServiceName\" AS service_name,
      \"ServiceCategory\" AS service_category,
      CAST(NULL AS VARCHAR) AS charge_sub_category,
      \"ChargeCategory\" AS charge_category,
      \"RegionId\" AS region_id,
      \"ResourceId\" AS resource_id,
      date_format(cast(\"ChargePeriodStart\" as timestamp), '%Y-%m-%dT%H:%i:%sZ') AS charge_period_start,
      date_format(cast(\"ChargePeriodEnd\" as timestamp), '%Y-%m-%dT%H:%i:%sZ') AS charge_period_end,
      SUM(cast(\"BilledCost\" as double)) AS billed_cost,
      SUM(cast(\"EffectiveCost\" as double)) AS effective_cost,
      SUM(cast(\"ConsumedQuantity\" as double)) AS consumed_quantity,
      \"ConsumedUnit\" AS consumed_unit,
      \"BillingCurrency\" AS billing_currency,
      \"BillingAccountId\" AS billing_account_id,
      \"SubAccountId\" AS sub_account_id,
      try(element_at(skupricedetails, 'InstanceType')) AS x_instance_type,
      try(element_at(skupricedetails, 'CoreCount')) AS x_vcpu,
      try(element_at(skupricedetails, 'MemorySize')) AS x_memory,
      CAST(NULL AS VARCHAR) AS x_gpu,
      arbitrary(\"ChargeDescription\") AS charge_description
    FROM
      \"$database\".\"$table\"
    WHERE
      $focus_tag_filter
      AND \"ChargePeriodStart\" >= DATE '$focus_initial_date'
      AND \"ChargePeriodStart\" <= DATE '$final_date'
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
      reason=$(aws athena get-query-execution --query-execution-id $executionId --query 'QueryExecution.Status.StateChangeReason' --output text 2>/dev/null || true)
      if [ -n "$reason" ] && [ "$reason" != "None" ]; then
        echo "Failure reason: $reason"
      fi
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
    row_count=$(jq 'length' "$OUTPUT_DIR/cloudcosts.json" 2>/dev/null || echo "0")
    if [ "$row_count" = "0" ]; then
      tag_display="${tagkey:-aws:eks:cluster-name}=$tagvalue"
      echo "Note: No cost rows matched (tag $tag_display in date range). Try --days to widen the range or confirm the tag exists in the FOCUS export."
    fi
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

  # When using FOCUS format, derive project from focusview (project.dataset.view_name) if not set
  if [[ "$format" == "focus" && "$focusview" != "" && "$projectname" = "" ]]; then
    projectname="${focusview%%.*}"
  fi

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
  # When format is focus we use focusview, not table; only require dataset/billingaccount for standard format
  if [[ "$table" = "" && "$format" != "focus" ]]; then
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

  /google-cloud-sdk/bin/bq --format=prettyjson --project_id "$projectname" query --max_rows=10000000 --nouse_legacy_sql "$sql" > "$OUTPUT_DIR/cloudcosts-tmp.json" && echo "Executing..."
  if grep -q "BigQuery error" "$OUTPUT_DIR/cloudcosts-tmp.json"; then
    echo "Error executing the query"
    cat "$OUTPUT_DIR/cloudcosts-tmp.json"
    exit 1
  fi

  if ! grep -q "\[" "$OUTPUT_DIR/cloudcosts-tmp.json"; then
    echo "No data found for the given tag value"
    cat "$OUTPUT_DIR/cloudcosts-tmp.json"
    exit 1
  fi

  echo "Google BigQuery finished..."
  sed -n '/^\[$/,$ p' "$OUTPUT_DIR/cloudcosts-tmp.json" > "$OUTPUT_DIR/cloudcosts-tmp-clean.json"

  mv "$OUTPUT_DIR/cloudcosts-tmp-clean.json" "$OUTPUT_DIR/cloudcosts.json"
  echo "Saved GCP costs file in $OUTPUT_DIR/cloudcosts.json"

  exit 0
fi
if [[ "$provider" == "azure" ]]; then
  echo "Azure Cost Management integration (FOCUS format)......"

  # Azure CLI config dir: use chart-provided path when set (shared with az-login init container), else writable OUTPUT_DIR
  if [[ -z "${AZURE_CONFIG_DIR:-}" ]]; then
    export AZURE_CONFIG_DIR="${OUTPUT_DIR}/.azure"
  fi
  mkdir -p "$AZURE_CONFIG_DIR"

  if [[ "$subscription" = "" ]]; then
    echo "Error: --subscription is required for Azure"
    usage
    exit 1
  fi

  # Azure cost data takes 24-72 hours to finalize
  # Apply 2-day lag to ensure complete data (ignore today and yesterday)
  lag_days=2
  initial_date=$(date_ago "$((days + lag_days))" "%Y-%m-%d")
  final_date=$(date_ago "$lag_days" "%Y-%m-%d")

  if [[ "$tagvalue" != "" ]]; then
    if [[ "$tagkey" = "" ]]; then
      tagkey="kubernetes-cluster"
    fi
    echo "Azure Cost Management query is running......"
    echo "Querying costs from $initial_date to $final_date for tag $tagkey=$tagvalue"
    # FOCUS format request body with tag filter
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
  else
    echo "Azure Cost Management query is running......"
    echo "Querying costs from $initial_date to $final_date (no tag filter)"
    # FOCUS format request body without tag filter
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
    ]
  }
}
EOF
)
  fi

  echo "(2-day lag applied to ensure cost data is finalized)"

  # Query Azure Cost Management API
  az rest --method post \
    --url "https://management.azure.com/subscriptions/$subscription/providers/Microsoft.CostManagement/query?api-version=2023-11-01" \
    --body "$request_body" \
    --output json > "$OUTPUT_DIR/cloudcosts-tmp.json"

  if [[ $? -ne 0 ]]; then
    echo "Error executing Azure Cost Management query"
    cat "$OUTPUT_DIR/cloudcosts-tmp.json"
    exit 1
  fi

  # Check for errors in response
  if grep -q '"error"' "$OUTPUT_DIR/cloudcosts-tmp.json"; then
    echo "Azure Cost Management API returned an error"
    cat "$OUTPUT_DIR/cloudcosts-tmp.json"
    exit 1
  fi

  row_count=$(jq '.properties.rows | length' "$OUTPUT_DIR/cloudcosts-tmp.json")
  if [[ "$row_count" -eq 0 && "$tagvalue" != "" ]]; then
    request_body_no_tag=$(echo "$request_body" | jq 'del(.dataset.filter)')
    # Fallback 1: dimension filter on ResourceId with AKS managed cluster resource ID(s)
    # Filter: dimensions.name=ResourceId, operator=In, values=[/subscriptions/.../resourceGroups/.../providers/Microsoft.ContainerService/managedClusters/...]
    echo "Tag filter (kubernetes-cluster=$tagvalue) returned no rows. Trying dimension filter on cluster ResourceId..."
    cluster_ids=$(az aks list --subscription "$subscription" --query "[?contains(name, '$tagvalue')].id" -o tsv 2>/dev/null || true)
    if [[ -n "$cluster_ids" ]]; then
      cluster_ids_json=$(echo "$cluster_ids" | tr '\t' '\n' | jq -R . | jq -s .)
      request_body_dim=$(echo "$request_body_no_tag" | jq --argjson ids "$cluster_ids_json" '.dataset.filter = { "dimensions": { "name": "ResourceId", "operator": "In", "values": $ids } }')
      az rest --method post \
        --url "https://management.azure.com/subscriptions/$subscription/providers/Microsoft.CostManagement/query?api-version=2023-11-01" \
        --body "$request_body_dim" \
        --output json > "$OUTPUT_DIR/cloudcosts-tmp.json" 2>/dev/null || true
      if ! grep -q '"error"' "$OUTPUT_DIR/cloudcosts-tmp.json" 2>/dev/null; then
        row_count=$(jq '.properties.rows | length' "$OUTPUT_DIR/cloudcosts-tmp.json")
        [[ "$row_count" -gt 0 ]] && echo "  Got $row_count rows from ResourceId dimension filter."
      fi
    fi
    # Fallback 2: server-side dimension filter on ResourceGroup (single API call)
    # https://learn.microsoft.com/en-us/rest/api/cost-management/query/usage - filter.dimensions.name=ResourceGroup, operator=In
    if [[ "$row_count" -eq 0 ]]; then
      echo "  Trying server-side ResourceGroup dimension filter..."
      rg_list=$(az group list --subscription "$subscription" --query "[?contains(name, '$tagvalue')].name" -o tsv 2>/dev/null || true)
      if [[ -n "$rg_list" ]]; then
        rg_json=$(echo "$rg_list" | tr '\t' '\n' | jq -R . | jq -s .)
        request_body_rg=$(echo "$request_body_no_tag" | jq --argjson rgs "$rg_json" '.dataset.filter = { "dimensions": { "name": "ResourceGroup", "operator": "In", "values": $rgs } }')
        az rest --method post \
          --url "https://management.azure.com/subscriptions/$subscription/providers/Microsoft.CostManagement/query?api-version=2023-11-01" \
          --body "$request_body_rg" \
          --output json > "$OUTPUT_DIR/cloudcosts-tmp.json" 2>/dev/null || true
        if ! grep -q '"error"' "$OUTPUT_DIR/cloudcosts-tmp.json" 2>/dev/null; then
          row_count=$(jq '.properties.rows | length' "$OUTPUT_DIR/cloudcosts-tmp.json")
          [[ "$row_count" -gt 0 ]] && echo "  Got $row_count rows from ResourceGroup dimension filter (server-side)."
        fi
      fi
    fi
    # Fallback 3: resource group scope (one API call per RG; all filtering still server-side)
    if [[ "$row_count" -eq 0 ]]; then
      echo "  Trying resource group scope (one query per RG, server-side)..."
      rg_list=$(az group list --subscription "$subscription" --query "[?contains(name, '$tagvalue')].name" -o tsv 2>/dev/null || true)
      if [[ -n "$rg_list" ]]; then
        all_rows="[]"
        columns_json=""
        while IFS= read -r rg; do
          [[ -z "$rg" ]] && continue
          echo "  Querying resource group: $rg"
          az rest --method post \
            --url "https://management.azure.com/subscriptions/$subscription/resourceGroups/$(echo "$rg" | sed 's/ /%20/g')/providers/Microsoft.CostManagement/query?api-version=2023-11-01" \
            --body "$request_body_no_tag" \
            --output json > "$OUTPUT_DIR/cloudcosts-rg-tmp.json" 2>/dev/null || true
          if [[ -f "$OUTPUT_DIR/cloudcosts-rg-tmp.json" ]] && ! grep -q '"error"' "$OUTPUT_DIR/cloudcosts-rg-tmp.json"; then
            rg_rows=$(jq -c '.properties.rows' "$OUTPUT_DIR/cloudcosts-rg-tmp.json")
            if [[ "$rg_rows" != "[]" && "$rg_rows" != "null" ]]; then
              [[ -z "$columns_json" ]] && columns_json=$(jq -c '.properties.columns' "$OUTPUT_DIR/cloudcosts-rg-tmp.json")
              all_rows=$(jq -n --argjson a "$all_rows" --argjson b "$rg_rows" '$a + $b')
            fi
          fi
        done <<< "$rg_list"
        if [[ -n "$columns_json" && "$all_rows" != "[]" ]]; then
          jq -n --argjson cols "$columns_json" --argjson rows "$all_rows" '{properties: {columns: $cols, rows: $rows}}' > "$OUTPUT_DIR/cloudcosts-tmp.json"
          row_count=$(jq '.properties.rows | length' "$OUTPUT_DIR/cloudcosts-tmp.json")
          echo "  Combined $row_count rows from resource group scope (server-side)."
        fi
        rm -f "$OUTPUT_DIR/cloudcosts-rg-tmp.json"
      fi
    fi
    # All filtering is server-side (tag, ResourceId dimension, ResourceGroup dimension, or RG scope). No local filtering of rows.
  fi
  if [[ "$row_count" -eq 0 ]]; then
    echo "Azure Cost Management API returned no rows. Check: date range (2-day lag), tag filter (e.g. kubernetes-cluster=$tagvalue), and that the subscription has usage in that range."
    echo "Response columns: $(jq -c '.properties.columns' "$OUTPUT_DIR/cloudcosts-tmp.json")"
  fi

  echo "Transforming to FOCUS format..."
  # Cost Management Query API does not return vCPU/memory/instance size; those appear in Cost Details (usage details) export or ARM resource properties.
  # Output FOCUS columns; Azure API does not return instance type/vCPU/memory so those are omitted (not null).
    jq --arg sub "$subscription" '
    .properties | (.columns | map(.name)) as $names |
    [.rows[] | ($names | to_entries | map({(.[1]): .[0]}) | add) as $idx |
      [.[$idx["Cost"] // $idx["PreTaxCost"]], .[$idx["UsageQuantity"]], (.[$idx["UsageDate"]] // .[$idx["PreTaxCost"]]), .[$idx["ServiceName"]], .[$idx["MeterCategory"]], .[$idx["MeterSubCategory"]], .[$idx["ResourceLocation"]], .[$idx["ResourceId"]], .[$idx["ResourceGroupName"]], .[$idx["ChargeType"]], .[$idx["PublisherType"]], .[$idx["Currency"]]] as $r |
    {
      ServiceName: $r[3],
      ServiceCategory: $r[4],
      ChargeSubCategory: $r[5],
      ChargeCategory: ($r[9] // "Usage"),
      RegionId: $r[6],
      ResourceId: $r[7],
      ChargePeriodStart: (($r[2] // $r[0] | tostring) | if length >= 8 then "\(.[0:4])-\(.[4:6])-\(.[6:8])T00:00:00Z" else empty end),
      ChargePeriodEnd: (($r[2] // $r[0] | tostring) | if length >= 8 then "\(.[0:4])-\(.[4:6])-\(.[6:8])T23:59:59Z" else empty end),
      BilledCost: $r[0],
      EffectiveCost: $r[0],
      ConsumedQuantity: $r[1],
      ConsumedUnit: null,
      BillingCurrency: $r[11],
      BillingAccountId: $sub,
      SubAccountId: $sub,
      ChargeDescription: ([$r[3], $r[4], $r[5]] | map(select(. != null and . != "")) | join(" - ")),
      ProviderName: "Microsoft Azure",
      PublisherName: (if $r[10] == "Azure" then "Microsoft Azure" else $r[10] end),
      x_ResourceGroupName: $r[8]
    }]' "$OUTPUT_DIR/cloudcosts-tmp.json" > "$OUTPUT_DIR/cloudcosts-focus.json" 2>/dev/null || {
    # Fallback: build row by column name. Azure returns rows as arrays; column order: Cost, UsageQuantity, UsageDate, ServiceName, MeterCategory, MeterSubCategory, ResourceLocation, ResourceId, ResourceGroupName, ChargeType, PublisherType, Currency (indices 0-11).
    jq --arg sub "$subscription" '[.properties.rows[] | select(type == "array") | (
      (.[0] // 0) as $cost |
      (.[1] // 0) as $qty |
      (.[2] | tostring) as $date |
      {
        ServiceName: .[3],
        ServiceCategory: .[4],
        ChargeSubCategory: .[5],
        ChargeCategory: (.[9] // "Usage"),
        RegionId: .[6],
        ResourceId: .[7],
        ChargePeriodStart: (if ($date | length) >= 8 then "\($date[0:4])-\($date[4:6])-\($date[6:8])T00:00:00Z" else empty end),
        ChargePeriodEnd: (if ($date | length) >= 8 then "\($date[0:4])-\($date[4:6])-\($date[6:8])T23:59:59Z" else empty end),
        BilledCost: $cost,
        EffectiveCost: $cost,
        ConsumedQuantity: $qty,
        ConsumedUnit: null,
        BillingCurrency: .[11],
        BillingAccountId: $sub,
        SubAccountId: $sub,
        ChargeDescription: ([.[3], .[4], .[5]] | map(select(. != null and . != "")) | join(" - ")),
        ProviderName: "Microsoft Azure",
        PublisherName: (if .[10] == "Azure" then "Microsoft Azure" else .[10] end),
        x_ResourceGroupName: .[8]  # index 8=ResourceGroupName, 9=ChargeType, 10=PublisherType, 11=Currency
      }
    )]' "$OUTPUT_DIR/cloudcosts-tmp.json" > "$OUTPUT_DIR/cloudcosts-focus.json"
  }

  if [[ $? -ne 0 ]]; then
    echo "Error transforming to FOCUS format"
    cat "$OUTPUT_DIR/cloudcosts-tmp.json"
    exit 1
  fi

  mv "$OUTPUT_DIR/cloudcosts-focus.json" "$OUTPUT_DIR/cloudcosts.json"
  rm -f "$OUTPUT_DIR/cloudcosts-tmp.json"

  echo "Azure Cost Management query finished..."
  echo "Saved Azure costs file in FOCUS format to $OUTPUT_DIR/cloudcosts.json"

  exit 0
fi
echo "--provider is required and should be aws, gcp, or azure"
exit 1