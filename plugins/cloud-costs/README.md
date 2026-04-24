# cloud-costs

Queries and saves cloud cost data for Fairwinds Insights. Supports AWS and GCP in standard format, and Azure in FOCUS (FinOps Open Cost and Usage Specification) format.

## Supported Providers

| Provider | Data Source | Authentication |
|----------|-------------|----------------|
| AWS | [Cost and Usage Report (CUR)](https://docs.aws.amazon.com/cur/latest/userguide/what-is-cur.html) and [CUR 2.0 / Data Exports](https://docs.aws.amazon.com/cur/latest/userguide/table-dictionary-cur2.html) via Athena | AWS IAM / Instance Role |
| GCP | [Cloud Billing Export](https://cloud.google.com/billing/docs/how-to/export-data-bigquery) via BigQuery | Service Account |
| Azure | [Cost Management API](https://learn.microsoft.com/en-us/rest/api/cost-management/) | Service Principal / Managed Identity |

## Usage

```bash
cloud-costs.sh \
  --provider <aws|gcp|azure> \
  --tagkey <tag-key> \
  --tagvalue <tag-value> \
  [--days <number>] \
  [provider-specific options...]
```

### Common Options

| Option | Description | Default |
|--------|-------------|---------|
| `--provider` | Cloud provider: `aws`, `gcp`, or `azure` | Required |
| `--tagkey` | Tag key to filter resources | Required for AWS, optional for GCP/Azure |
| `--tagvalue` | Tag value to filter resources | Required |
| `--days` | Number of days to query | `5` |
| `--timeout` | Query timeout in seconds | `60` |

### AWS Options

| Option | Description | Required |
|--------|-------------|----------|
| `--database` | Athena database name | Yes |
| `--table` | Athena table name (or view) | Yes |
| `--catalog` | Athena catalog | Yes |
| `--workgroup` | Athena workgroup | Yes |
| `--tagprefix` | Legacy CUR only: flattened tag column prefix (e.g. `resource_tags_user_`) | No |
| `--awscurversion` | `legacy` (default) or `cur2` for native [CUR 2.0](https://docs.aws.amazon.com/cur/latest/userguide/table-dictionary-cur2.html) schema in Athena | No |
| `--cur2tagcolumn` | CUR 2.0 only: tag map column to filter — `tags` (default) or `resource_tags` | No |
| `--tagmapkey` | CUR 2.0 only: exact map key for the tag filter (overrides `--tagkey` / `resourceTags/` default). Example: `resourceTags/kubernetes-cluster` | No |

Environment variable `CLOUD_COSTS_AWS_CUR_VERSION` sets the same value as `--awscurversion` when the flag is omitted (`legacy` or `cur2`).

**Legacy CUR** (`--awscurversion legacy`, default): use `--tagprefix` for flattened `resource_tags_user_*` columns, or omit it to filter on a `tags` MAP using raw `--tagkey`.

**CUR 2.0** (`--awscurversion cur2`): `--tagprefix` is ignored. The query filters the unified `tags` MAP with `resourceTags/<tagkey>` (and falls back to `<tagkey>`), or use `--cur2tagcolumn resource_tags` with cost-allocation tag keys, or set `--tagmapkey` to the full key. Product fields that live in the CUR 2.0 `product` map (`memory`, `vcpu`, `clock_speed`, `gpu`) are read via `element_at`; top-level `product_instance_type` and `product_product_family` are unchanged. Output column names match legacy so [Fairwinds Insights](https://github.com/fairwindsops/fairwinds-insights) `AwsCostsReport` parsing continues to work.

### GCP Options

| Option | Description | Required |
|--------|-------------|----------|
| `--projectname` | GCP project name | Yes |
| `--dataset` | BigQuery dataset name | If `--table` not provided |
| `--billingaccount` | GCP billing account ID | If `--table` not provided |
| `--table` | Full BigQuery table path | No (auto-generated if not provided) |

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

### AWS - CUR 2.0 (Data Exports) in Athena

Use when the Athena table is the native CUR 2.0 schema (nested `tags`, `product` MAP, etc.):

```bash
cloud-costs.sh \
  --provider aws \
  --awscurversion cur2 \
  --tagkey "kubernetes-cluster" \
  --tagvalue "my-cluster" \
  --database "cur2_database" \
  --table "cur2_export" \
  --catalog "AwsDataCatalog" \
  --workgroup "primary" \
  --days 7
```

If your cost allocation tag appears under a non-default key in the `tags` map, pass the full key:

```bash
cloud-costs.sh \
  --provider aws \
  --awscurversion cur2 \
  --tagkey "kubernetes-cluster" \
  --tagvalue "my-cluster" \
  --tagmapkey "resourceTags/kubernetes-cluster" \
  --database "cur2_database" \
  --table "cur2_export" \
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

### Azure (FOCUS format only)

```bash
cloud-costs.sh \
  --provider azure \
  --subscription "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" \
  --tagkey "kubernetes-cluster" \
  --tagvalue "my-cluster" \
  --days 7
```

> **Note:** Azure always outputs FOCUS format.
>
> **Data Lag:** Azure applies a 2-day lag automatically (ignores today and yesterday) to ensure cost data is fully finalized.

### Filter Azure costs to one cluster (server-side only)

All Azure filtering is done **on the server** via the [Cost Management Query API](https://learn.microsoft.com/en-us/rest/api/cost-management/query/usage). The script never filters rows locally.

Pass the cluster tag so the API returns only that cluster’s costs. If omitted, `--tagkey` defaults to `kubernetes-cluster`.

```bash
cloud-costs.sh \
  --provider azure \
  --subscription "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" \
  --tagkey "kubernetes-cluster" \
  --tagvalue "james-azure-cluster" \
  --days 7
```

Resources must be tagged with that key/value in Azure for the tag filter to apply. If the tag filter returns no rows, the script tries server-side fallbacks in order:

1. **Tag filter** – `dataset.filter.tags` (tag key/value).
2. **ResourceId dimension** – `dataset.filter.dimensions` with AKS cluster resource IDs.
3. **ResourceGroup dimension** – one API call with `filter.dimensions.name=ResourceGroup`, `operator=In`, values = resource groups whose name contains the cluster.
4. **Resource group scope** – one API call per matching resource group (scope = `/subscriptions/.../resourceGroups/{name}`).

Output is only what the API returns; no client-side filtering is applied.

## Output Formats

### AWS and GCP (standard format)

- **AWS**: Athena `ResultSet` with rows and column metadata
- **GCP**: BigQuery JSON array

### Azure (FOCUS format)

Azure returns a unified JSON array following the [FOCUS specification](https://focus.finops.org/):

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

### Azure FOCUS columns

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

### Azure FOCUS: vCPU and memory

The Azure **Cost Management Query API** used by this plugin returns cost and usage by dimensions (service, meter category, resource ID, etc.) but **does not return vCPU count, memory size, or instance type** (VM size). Those fields are output as `null` in FOCUS so the schema matches AWS.

To get vCPU/memory/instance size for Azure costs you would need one of:

- **Cost Details (usage details) export** – Azure’s FOCUS cost and usage details files (e.g. from the portal or Cost Details API) include SKU/meter info; instance size can sometimes be inferred from meter names (e.g. "D4s v3").
- **Resource Manager (ARM)** – For a given `ResourceId`, call the compute/VMs API to read the VM’s `hardwareProfile.vmSize`, then map that to vCPUs and memory using [Azure VM sizes](https://learn.microsoft.com/en-us/azure/virtual-machines/sizes).

### GCP cluster filtering

GCP rows are included if either:

- The resource has the cluster tag (e.g. `goog-k8s-cluster-name` = `my-cluster`), or
- The GKE resource ID contains the cluster name (e.g. `.../clusters/my-cluster`), so control plane and clusters without cost-allocation tags are included.

If `--tagkey` is omitted for GCP, it defaults to `goog-k8s-cluster-name`.

## Authentication

### AWS
- Uses AWS CLI default credential chain
- Requires permissions: `athena:StartQueryExecution`, `athena:GetQueryExecution`, `athena:GetQueryResults`, `s3:GetObject`

### GCP
- Uses `gcloud` default credentials or service account
- Requires BigQuery read permissions on the billing export dataset

### Azure
- Uses Azure CLI authentication. In Kubernetes the container cannot run interactive `az login`; use one of:
  - **Service principal** (recommended for CronJobs): Set env vars `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`, and `AZURE_TENANT_ID`. The script will run `az login --service-principal` automatically.
  - **Managed identity** (e.g. AKS with workload identity): Set `AZURE_USE_MANAGED_IDENTITY=true` (and `AZURE_CLIENT_ID` if needed); the script runs `az login --identity`.
  - **Local use**: Run `az login` on your machine before running the script.
- The service principal or identity must have **Cost Management Reader** (or broader) role on the subscription.
