# realtime-reporter

## Run

You'll need to provide a configuration file for the reporter. An example configuration is located at `examples/realtime-reporter.yaml`

The Insights authentication token is passed as an environment variable. This is required:

```
export FAIRWINDS_TOKEN=$TOKEN
```

```
 go run main.go \
    --organization acme-co \
    --cluster kind \
    --host http://192.168.1.27:3001 \
    --config examples/realtime-reporter.yaml
```

## Example Output

* resource added

```json
{"event_version":1,"timestamp":1702656153316168000,"kube_event":"add","kind":"Namespace","namespace":"default","workload":"nginx-deployment","data":{"Contents":"B64_ENCODED_REPORT","Report":"polaris","Version":"1.0"}}
```

* resource updated

```json
{"event_version":1,"timestamp":1702058147263443000,"kube_event":"update","kind":"Deployment","namespace":"default","workload":"nginx-deployment","data":{"Contents":"B64_ENCODED_REPORT","Report":"polaris","Version":"1.0"}}
```

* resource deleted

```json
{"event_version":1,"timestamp":1702058166269670000,"kube_event":"delete","kind":"Deployment","namespace":"default","workload":"nginx-deployment","data":null}
```
