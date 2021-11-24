# Right-Sizer

This Insights plugin is experimental. Please also see [the to-do file](./TODO.md) for known issues and future plans.

Right-sizer is a Kubernetes Controller that detects containers that have been OOM-killed, and maintains an Insights report of those containers with their owning pod-controllers such as Deployments, StatefulSets, or DaemonSets. The Controller persists report data in a Kubernetes ConfigMap, which the accompanying Insights agent CronJob retrieves and submits to the Insights API.

The right-sizer controller removes pod-controllers from its report

* Every 24 HRs (currently hard-coded)
* IF we receive an event that involves the pod-controller. These currently are indirect events, such as a ReplicaSet scaling a deployment because that deployment has changed. The `ResourceVersion` field of the report item and the new in-cluster object is compared to determine whether related items (same pod-controller, multiple containers) should be deleted from the report.

## Debugging

For debugging, the current Insights report can be retrieved from a right-sizer controller pod via `http://{pod IP}:8080/report`. This HTTP server is a vestige of a previous design, but is kept around for troubleshooting, and eventual Kubernetes readiness and liveness probes.

The right-sizer controller pod can be run with the `-v X` argument to increase verbosity - using `X` of 1 or 2 temporarily will get sufficient detail. NOTE that increasing verbosity beyond 2 will log all events the controller sees, this could be undesirable in a busy cluster.

## Running locally

To test the right-sizer controller locally using a [kind](https://kind.sigs.k8s.io/) cluster, either:

* Build and run the right-sizer controller locally:

```bash
go build -o right-sizer cmd/main.go
./right-sizer
```

* Build and run the right-sizer controller in a container:

```bash
docker build -t right-sizer .
docker run --network host \
  -e "KUBECONFIG=/local/kubeconfig" \
  -v $HOME/.kube/config:/local/kubeconfig \
  right-sizer
```

