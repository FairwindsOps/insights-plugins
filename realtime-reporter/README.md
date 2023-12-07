# realtime-reporter

## Run

You'll need to provide a configuration file for the reporter. The default location will be at `~/.insights-reporter.yaml`, or you can specify a file as a command line argument.

Example configuration:

```yaml
resources:
  -  apps/v1/deployments
  -  apps/v1/statefulsets
  -  v1/configmaps
  -  apps/v1/daemonsets
  -  batch/v1/jobs
  -  v1/pods
  -  v1/services
  -  networking.k8s.io/v1/ingresses
  -  v1/nodes
  -  v1/namespaces
  -  v1/persistentvolumes
  -  v1/persistentvolumeclaims

namespaces:
  - all
```

From this `insights-plugins/realtime-reporter` directory, you can run simply with `go run main.go`

## Example Output

* resource created

```json
{"version":1,"timestamp":"1701121805498107000","namespace":"default","workload":"nginx-deployment","data":{"Contents":"B64_ENCODED_REPORT","Report":"polaris","Version":"1.0"}}
```

* resource updated

```json
{"version":1,"timestamp":"1701121878837615000","namespace":"insights-agent","workload":"workloads-28352003","data":{"Contents":"B64_ENCODED_REPORT","Report":"polaris","Version":"1.0"}}
```

* resource deleted

```json
{"version":1,"timestamp":"1701121778054851000","namespace":"default","workload":"nginx-deployment","data":null}
```

## TODOs

* Use the configuration provided by the `insights-agent` helm chart
* Support uploading to the new endpoint once it's created
* Implement more report types
* Is `ReportJob` from the admission plugin the right thing to use here for incremental polaris reports? `Content` is represented as `[]byte` which will encode to base64 by the standard json library
