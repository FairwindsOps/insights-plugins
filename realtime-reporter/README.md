# realtime-reporter

## Run

You'll need to provide a configuration file for the reporter. The default location will be at `~/.insights-reporter.yaml`, or you can specify a file as a command line argument.

Example configuration:

```yaml
resources:
  -  v1/pods
  -  v1/services
  -  apps/v1/deployments
  -  apps/v1/statefulsets
  -  networking.k8s.io/v1/ingresses
  -  v1/nodes
  -  v1/namespaces
  -  v1/persistentvolumes
  -  v1/persistentvolumeclaims
  -  v1/configmaps
  -  apps/v1/daemonsets
  -  batch/v1/jobs

namespaces:
  - all
```

From this `insights-plugins/realtime-reporter` directory, you can run simply with `go run main.go`

## TODOs

* Use the configuration provided by the `insights-agent` helm chart
* Support uploading to the new endpoint once it's created
* Implement more report types
