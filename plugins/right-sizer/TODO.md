# Right-Sizer Plugin To-Dos

As this plugin is experimental, there are a higher amount of to-do items and possibilities - these are documented here for now.

## Plugin Known Issues / Planned Fixes

### General

* Fill out the `results.schema` file for this plugin.

### Report / Action Item

* Clean up the right-sizer report action item description to use a more friendly time display format, and not show both first and last OOM if only one OOM has been seen.. Also bold the number of OOMs to draw user attention.

### Controller

* Do we want to keep adding Kube events to pods where an OOM-kill is detected? This is the original purpose of the Kube-OOM-Event-Generator code, on which this controller is based.
* Should we directly monitor create/update/delete operations to catch changes to pod-controllers to remove their action items? Currently the event stream we listen to, catches transient events (ReplicaSet scaling when a deployment happened to get updated).
* Handle OOM-killed cron jobs.
	* CronJob pods do not have subsequent "container started" events, which we use as our trigger to inspect the previous "reason for container death" for other pod-controllers.
	* One CronJob idea is to inspect Job resources for all pods having failed, then inspect those pods for OOMKills?
* Add CLI options for
	* The state ConfigMap namespace and name
	* The `too old age` interval (when action items should be aged out that we haven't heard from)
	* The HTTP listen port (server serves the report and will be used for Kube readiness/liveness probes)
* Require a minimum number of OOM-kills before acting on a pod-controller.
* Finish feature to update memory-limits for the owning pod-controller of an OOM-killed pod.
* Provide enough useful information in an action item to help customers identify the correct area of a custom resource manifest which may manage multiple pod-controllers (Deployments or other CRDs). Likely including Pod labels in the action item, will provide enough useful information. We may be able to reliably understand/modify memory limits of some CRDs, but the complexity of some is not worth our risk.
	* For example, the [Vitess database operator](https://vitess.io/docs/get-started/operator/) has a `VitessCluster` CRD that manages multiple groups of Pods using multiple Deployments and a `EtcdLockserver` CRD.
		* The Pod labels include enough background for the customer to map an OOM-kill to the area of the parent `VitessCluster` resource that the customer should adjust.
		* Examples of information available in pod labels include the Vitess cluster name, cell/zone, and component (etcd, vtctld, vtgateway).
		* Within the owning `VitessCluster` pod-controller, limits can be set in a variety of places that inform the child managed pod-controller resources. For example:
			* spec -> partitionings -> {partition equality list} -> shardTemplate -> tabletPools -> cell -> mysqld
			* spec -> cells -> {cell name} -> gateway -> vttablet
	* Q: Should all of these labels be included in the differentiating information of an action item? Current differentiating information is Kube resource kind, resource namespace, resource name, and OOM-killed container name.

## Future Ideas / Consideration

### Controller

* Revamp storage of controller state (repopulate report when the controller dies or is restarted):
	* Don't store state at all, change Insights architecture to support submitting report items without requiring a full report to be submitted each time.
	* Store state in a Secret instead of a ConfigMap? Could be a better RBAC story, if limited to our namespace.
	* Store controller state in a CRD instead (still lives in Etcd but a "more proper architecture")?
	* Store state in persistent storage (StatefulSet) instead?
	* Additional protections around loading a state ConfigMap that a user may have tampered with? Encrypt with a static key? Use a checksum?
	* If we frequently update the ConfigMap or it gets larger, we *could* impact Etcd. E.G. Argo can have this problem with a significant scale of Argo workflows (each using a compressed/encoded Configmap).
* Require Prometheus and kube-state-metrics, and use that to obtain OOM-kill information via kube-state-metrics?
	* We still may not get the level of completeness we would on our own (controller watching events and in-cluster changes).

## Code-level Considerations

* Definitely add tests - the only tests are currently from the original Kubernetes OOM Event Generator project. I haven't added tests so far, to speed proof-of-concept development.

### General Layout Questions

* Move code from `src` to `pkg` sub-directory?
* Minimize `cmd/main.go` by moving more code into packages?

### Report Package Questions

* Replace `COntext.TODO()` with another context? What (controller vs. report operations) should share a context?

### Kubernetes-OOM-event-generator Code Questions

* Where/how to give credit, that we’ve used this code in our plugin??? https://github.com/xing/kubernetes-oom-event-generator
* Should `reportBuilder` be a member of the controller struct? I think this is the cleanest way to maintain a report in the controller without polluting the controller code. I may end up passing the ReportBuilder into the NewController() struct, to facilitate command-line flags.
* I overloaded `util.Clientset()` to return `client, dynamicClient, RESTMapper` because all three of those things require a KubeConfig. Should this be refactored?
* Use less temporary variables when constructing a report item, in `evaluatePodStatus`?
