# realtime-reporter

## Run

You'll need to provide a configuration file for the reporter. The default location will be at `~/.insights-reporter.yaml`, or you can specify a file as a command line argument.

Example configuration:

```yaml
resources:
  - apps/v1/deployments
  - apps/v1/statefulsets
  - v1/configmaps
  - apps/v1/daemonsets
  - batch/v1/jobs
  - v1/pods
  - v1/services
  - networking.k8s.io/v1/ingresses
  - v1/nodes
  - v1/namespaces
  - v1/persistentvolumes
  - v1/persistentvolumeclaims
  - apps/v1/replicasets

namespaces:
  - all
```

From this `insights-plugins/realtime-reporter` directory, you can run simply with `go run main.go`

## Example Output

* resource created

```json
{"event_version":1,"timestamp":1702058101532424000,"kind":"Deployment","namespace":"local-path-storage","workload":"local-path-provisioner","data":{"Contents":"B64_ENCODED_REPORT","Report":"polaris","Version":"1.0"}}
```

* resource updated

```json
{"event_version":1,"timestamp":1702058147263443000,"kind":"Deployment","namespace":"default","workload":"nginx-deployment","data":{"Contents":"B64_ENCODED_REPORT","Report":"polaris","Version":"1.0"}}
```

* resource deleted

```json
{"event_version":1,"timestamp":1702058166269670000,"kind":"Deployment","namespace":"default","workload":"nginx-deployment","data":null}
```

## TODOs

* Use the configuration provided by the `insights-agent` helm chart
* Support uploading to the new endpoint once it's created
* Implement more report types
* Is `ReportJob` from the admission plugin the right thing to use here for incremental polaris reports? `Content` is represented as `[]byte` which will encode to base64 by the standard json library
