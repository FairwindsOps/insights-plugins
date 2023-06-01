# Workload

Retrieves metadata about running workloads in the current cluster, this includes pods and controllers like deployments. This also retrieves namespace and node names and annotations.

# Updating version

After a version update on `version.txt`, update the reference on `pkg/version.txt` manually or by running:

> go generate pkg/version.go
