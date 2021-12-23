# Right-Sizer

This Insights plugin is experimental. Please also see [the to-do file](./TODO.md) for known issues and future plans.

Right-sizer is a Kubernetes Controller that detects and optionally modifies containers that have been OOM-killed, and maintains an Insights report of those containers with their owning pod-controllers (such as Deployments, StatefulSets, or DaemonSets). The Controller persists report data in a Kubernetes ConfigMap, which the accompanying Insights agent CronJob retrieves and submits to the Insights API.

The right-sizer controller removes pod-controllers from its report

* When no OOM-kill has been seen within a time window (default 24 HRs).
* If the memory limits have been modified compared to the limits from the last-seen OOM-kill.
	* This is realized when the controller sees an indirect event (ReplicaSet scaling) related to the pod-controller, which triggers the controller to fetch the pod-controller resource and compare its limits.

## Enabling Updating of Memory Limits
 
Updating memory limits is disabled by default. When enabling this feature, you may want to also specify a list of Kubernetes namespaces to limit potential action-item noise, and impact to workloads.

The [insights-agent chart values](https://github.com/FairwindsOps/charts/blob/master/stable/insights-agent/values.yaml) include `rightsizer.updateMemoryLimits` values for enabling and configuring this feature. The Helm chart values impact the right-sizer controller command-line options.

Available configuration includes:

* The minimum number of OOM-kills a container must have, before memory limits are updated by patching its pod-controller.
* The limits increment, which is multiplied by the current (OOM-killed) container limits to calculate the new limits to be updated.
* The maximum limits, which is multiplied by the limits of the first-seen OOM-kill, to calculate the highest value to which limits can be updated for that container.
* Namespaces which limit both where OOM-kills are considered, and where memory limits will be updated.

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

