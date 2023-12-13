# realtime-reporter

## Run

You'll need to provide a configuration file for the reporter. An example configuration is located at `examples/insights-reporter.yaml`

The Insights authentication token is passed as an environment variable. This is required:

```
export FAIRWINDS_TOKEN=$TOKEN
```

```
 go run main.go \
    --organization acme-co \
    --cluster kind \
    --host http://192.168.1.27:3001 \
    --config examples/insights-reporter.yaml
```

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

* Implement more report types
* Is `ReportJob` from the admission plugin the right thing to use here for incremental polaris reports? `Content` is represented as `[]byte` which will encode to base64 by the standard json library
