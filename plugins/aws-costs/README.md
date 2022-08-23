# aws-costs

AWS costs plugin is built on [AWS costs and Usage Report](https://docs.aws.amazon.com/cur/latest/userguide/what-is-cur.html).

First step is to create the Athena infrastructure using Terraform, CloudFormation, etc. Fairwinds provides Terraform config.

The CUR report is created by AWS and stored in AWS S3.
Basically on the Athena process AWS collects CUR data from S3 and make it available as a SQL table that can be queried.

"Athena is out-of-the-box integrated with AWS Glue Data Catalog".
If you go to AWS Glue you can see there the infrastructure previously created to connect S3 CUR data into Athena.

Plugin Usage:
awscosts \
  --database < database name> \
  --table < table name> \
  --tagkey < tag key> \
  --tagvalue < tag value> \
  --catalog < catalog> \
  --workgroup < workgroup> \
  [--timeout < time in seconds>]

* **database name**: the database created on AWS Glue Data. ex: athena_cur_database
* **table name**: aws cur report name. Ex: fairwinds_insights_cur_report
* **tagkey**: tag key is the tag used on EC2 to indicate that it's a cluster node. Ex: KubernetesCluster (in case of Kops). The column name in Athena has a prefix resource_tags_user_. Also AWS applies pascal camel to split the tag name. In this example the column in Athena will be: resource_tags_user_kubernetes_cluster.
* **tagvalue**: the value associated to the tag for filtering. Ex: production, staging
* **catalog**: default AWS Glue Catalog is AwsDataCatalog
workgroup: workgroup created on Athena to be used on querying
