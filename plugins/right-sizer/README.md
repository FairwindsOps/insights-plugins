# Right-Sizer

Right-sizer is a Kubernetes Controller that detects containers that have been OOM-killed, and maintains an Insights report of the owning pod-controllers such as Deployments, StatefulSets, or DaemonSets. This Controller persists report data in a Kubernetes ConfigMap, which the accompanying Insights agent CronJob retrieves and submits to the Insights API.

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

