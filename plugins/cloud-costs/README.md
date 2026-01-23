# cloud-costs

Queries and saves cloud cost data for Fairwinds Insights. Supports AWS, GCP, and Azure with optional FOCUS (FinOps Open Cost and Usage Specification) format output.

## Supported Providers

| Provider | Data Source | Authentication |
|----------|-------------|----------------|
| AWS | [Cost and Usage Report (CUR)](https://docs.aws.amazon.com/cur/latest/userguide/what-is-cur.html) via Athena | AWS IAM / Instance Role |
| GCP | [Cloud Billing Export](https://cloud.google.com/billing/docs/how-to/export-data-bigquery) via BigQuery | Service Account |
| Azure | [Cost Management API](https://learn.microsoft.com/en-us/rest/api/cost-management/) | Service Principal / Managed Identity |

## Usage

```bash
cloud-costs.sh \
  --provider <aws|gcp|azure> \
  --tagkey <tag-key> \
  --tagvalue <tag-value> \
  [--format <standard|focus>] \
  [--days <number>] \
  [provider-specific options...]
```

### Common Options

| Option | Description | Default |
|--------|-------------|---------|
| `--provider` | Cloud provider: `aws`, `gcp`, or `azure` | Required |
| `--tagkey` | Tag key to filter resources | Required for AWS, optional for GCP/Azure |
| `--tagvalue` | Tag value to filter resources | Required |
| `--format` | Output format: `standard` or `focus` (AWS/GCP only) | `standard` |
| `--days` | Number of days to query | `5` |
| `--timeout` | Query timeout in seconds | `60` |

### AWS Options

| Option | Description | Required |
|--------|-------------|----------|
| `--database` | Athena database name | Yes |
| `--table` | Athena table name | Yes |
| `--catalog` | Athena catalog | Yes |
| `--workgroup` | Athena workgroup | Yes |
| `--tagprefix` | Tag prefix (e.g., `resource_tags_user_`) | No |

### GCP Options

| Option | Description | Required |
|--------|-------------|----------|
| `--projectname` | GCP project name | Yes |
| `--dataset` | BigQuery dataset name | If `--table` not provided |
| `--billingaccount` | GCP billing account ID | If `--table` not provided |
| `--table` | Full BigQuery table path | No (auto-generated if not provided) |
| `--focusview` | FOCUS view name | Required if `--format focus` |

### Azure Options

| Option | Description | Required |
|--------|-------------|----------|
| `--subscription` | Azure subscription ID | Yes |

## Examples

### AWS - Standard Format

```bash
cloud-costs.sh \
  --provider aws \
  --tagprefix "resource_tags_user_" \
  --tagkey "kubernetes-cluster" \
  --tagvalue "my-cluster" \
  --database "cur_database" \
  --table "cost_and_usage_report" \
  --catalog "AwsDataCatalog" \
  --workgroup "primary" \
  --days 7
```

### AWS - FOCUS Format

```bash
cloud-costs.sh \
  --provider aws \
  --format focus \
  --tagprefix "resource_tags_user_" \
  --tagkey "kubernetes-cluster" \
  --tagvalue "my-cluster" \
  --database "cur_database" \
  --table "cost_and_usage_report" \
  --catalog "AwsDataCatalog" \
  --workgroup "primary"
```

### GCP - Standard Format

```bash
cloud-costs.sh \
  --provider gcp \
  --projectname "my-project" \
  --dataset "billing_export" \
  --billingaccount "XXXXXX-XXXXXX-XXXXXX" \
  --tagvalue "my-cluster" \
  --days 7
```

### GCP - FOCUS Format

```bash
cloud-costs.sh \
  --provider gcp \
  --format focus \
  --projectname "my-project" \
  --focusview "my-project.billing_export.focus_view" \
  --tagvalue "my-cluster"
```

### Azure (FOCUS format only)

```bash
cloud-costs.sh \
  --provider azure \
  --subscription "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" \
  --tagkey "kubernetes-cluster" \
  --tagvalue "my-cluster" \
  --days 7
```

> **Note:** Azure always outputs FOCUS format. The `--format` option is not applicable for Azure.
> 
> **Data Lag:** Azure applies a 2-day lag automatically (ignores today and yesterday) to ensure cost data is fully finalized.

## Output Formats

### Standard Format

Each provider returns its native format:
- **AWS**: Athena `ResultSet` with rows and column metadata
- **GCP**: BigQuery JSON array

### FOCUS Format

All providers return a unified JSON array following the [FOCUS specification](https://focus.finops.org/):

```json
[
  {
    "BilledCost": 2.19,
    "EffectiveCost": 2.19,
    "ConsumedQuantity": 24.0,
    "ChargePeriodStart": "2024-01-21T00:00:00Z",
    "ChargePeriodEnd": "2024-01-21T23:59:59Z",
    "ServiceName": "Virtual Machines",
    "ServiceCategory": "Compute",
    "RegionId": "us-east-1",
    "ResourceId": "/subscriptions/.../vm-001",
    "ChargeCategory": "Usage",
    "BillingCurrency": "USD",
    "ProviderName": "Microsoft Azure",
    "BillingAccountId": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
  }
]
```

## FOCUS Columns

| Column | Description |
|--------|-------------|
| `BilledCost` | The amount billed for the charge |
| `EffectiveCost` | The effective cost after discounts |
| `ConsumedQuantity` | The amount of usage consumed |
| `ConsumedUnit` | The unit of measure |
| `ChargePeriodStart` | Start of the charge period (ISO8601) |
| `ChargePeriodEnd` | End of the charge period (ISO8601) |
| `ServiceName` | The name of the cloud service |
| `ServiceCategory` | The category of the service |
| `ChargeCategory` | The type of charge (Usage, Tax, etc.) |
| `RegionId` | The region identifier |
| `ResourceId` | The unique resource identifier |
| `BillingCurrency` | The billing currency |
| `ProviderName` | The cloud provider name |
| `BillingAccountId` | The billing account identifier |

## GCP FOCUS View Setup

For GCP FOCUS format, you need to create a view in BigQuery using the provided SQL template:

1. Copy `gcp-FOCUS-query.sql` to your BigQuery console
2. Replace the placeholders:
   - `${BILLING_EXPORT_TABLE}` - Your billing export table path
   - `${PRICING_EXPORT_TABLE}` - Your pricing export table path
   - `${PRICING_EXPORT_DATE}` - Date for pricing data
3. Create the view in BigQuery
4. Use `--focusview` to reference the created view

## Authentication

### AWS
- Uses AWS CLI default credential chain
- Requires permissions: `athena:StartQueryExecution`, `athena:GetQueryExecution`, `athena:GetQueryResults`, `s3:GetObject`

### GCP
- Uses `gcloud` default credentials or service account
- Requires BigQuery read permissions on the billing export dataset

### Azure
- Uses Azure CLI authentication (`az login`)
- Requires `Cost Management Reader` role on the subscription
