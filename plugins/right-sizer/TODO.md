# Right-Sizer Plugin To-Dos

As this plugin is experimental, there are a higher amount of to-do items and possibilities - these are documented here for now.

## Known Issues / Planned Features

These are issues or features that have ben discussed, but not yet prioritized.

### UI / Action Item

* Clean up the right-sizer report action item description to use a more friendly time display format, and not show both first and last OOM if only one OOM has been seen.. Also bold the number of OOMs to draw user attention.

### Controller

* Bug: Crash-loop-backoffs due to OOM-kills are not detected, likely because the Kube event is not yet "container started", which is what the controller currently looks for.
	* OOM-kills which trigger CrashLoopBackoff do have an `InvolvedObject` in a Backoff event, however repeated BackOff events incorrectly inflate the number of OOM-kills.
	* When a pod recovers from CrashLoopBackOff, there is no ContainerStarted event to register the previous OOM-kill that triggered the BackOff.
* Feature: Add `minimum OOMs` option before an action-item is created. Currently this option exists for updating limits, but action-items are created when the first OOM-kill is seen.
* Feature: Allow configurable annotations to be added to pod-controllers that we update.
	* This is helpful I.E. temporarily have Flux ignore something we modified; avoid Gitops reverting our changes. `fluxcd.io/ignore: true`
* Bug: CronJobs do not work, because their pods are "run once."
	* CronJob pods do not have subsequent restarts nor "container started" events, which we use as our trigger to inspect the previous "reason for container death" for other pod-controllers.
	* One CronJob idea is to inspect Job resources for all pods having failed, then inspect those pods for OOMKills?
* Question: Do we want to keep adding Kube events to pods where an OOM-kill is detected? This is the original purpose of the Kube-OOM-Event-Generator code, on which this controller is based.
* Theory (not tested): OOM-killed pods that are a previous replica/generation (still being rolled) could falsely increase the OOM-kill count for the owning pod-controller.
	* Before acting on an OOM-killed pod, determine whether we have it's pod-controller in our report, and if so determine whether the pod generation is older than the latest one we stored in the report item.
		* This might not work because we don't store every generation update for a pod-controller.

## Feature / Improvement Ideas

These have not yet been priority for the initial POC phase, but are important improvements to code, scalability, and usability, captured here.

### Controller Architecture

#### Add Leader Election

Currently the controller can only run as a single replica. Once leader election has been added:


* Break out the readiness probe to its own HTTP path, so only the active pod is accessed via a potential Kube Service. A Service is helpful to scrape prometheus metrics, or accessing in-memory report data (`/report`).
* Update the Helm chart to include:
	* Setting replicas and/or HPA and PDB.
	* Add nodeSelector, affinity, and tolerations.

#### Use Work Queue(s)

Currently these things happen in real-time as Kube events are processed:

* Update memory limits of the owning pod-controller of an OOM-killed pod.
* When an event (ReplicaSet scaling) is seen related to a resource in our report, determine if the memory limit is modified from our updated value, then remove that report item.

A work queue could be used to process these things, as well as:

* Remove report items in batches, including both the above case (someone modified memory limits), and the item aged out of a time window.

#### Add Prometheus Metrics

The original OOM Event Generator code included prometheus metrics that can be scraped from the controller pod. Update these to include what our controller does. Some ideas:

* numOOMs seen for a pod-controller resource.
* When we add a report item (replace current "OOM-kill detected for a pod").
* When we update memory.
* Report item removed because someone else updated its memory limits.
* Report item aged out (time window).

#### Revamp Storage of Controller State

The controller state is stored in a ConfigMap to repopulate report data when the controller pod is restarted). These are some ideas for changing this design:ƒ

* Don't store state at all, change Insights architecture to support submitting report items without requiring a full report to be submitted each time.
* Store state in a Secret instead of a ConfigMap? Could be a better RBAC story, if limited to our namespace.
* Store controller state in a CRD instead (still lives in Etcd but a "more proper architecture")?
* Store state in persistent storage (StatefulSet) instead? ConfigMaps do have a 1M size limit.
* Additional protections around loading a state ConfigMap that a user may have tampered with? Encrypt with a static key? Use a checksum?
* If we frequently update the ConfigMap or it gets larger, we *could* impact Etcd. E.G. Argo can have a comparable problem with a significant scale of Argo workflows (each using a compressed/encoded ConfigMap).

#### Realizing a Right-Sizer-Modified Pod-controller has Changed

Should we directly monitor create/update/delete operations to catch changes to pod-controllers (user changed memory limits) that invalidate their action items? Currently the event stream we listen to, catches transient events (ReplicaSet scaling when a deployment was updated).

* A challenge with directly monitoring pod-controllers that are report items, is we can't know-to-monitor custom resources a customer may be using. This is an advantage of using transient events where known controllers (like Deployment or ReplicaSet) are part of the chain of OwnerReference.

#### Get OOM-kill Info Another Way

Currently OOM-kills are realized via Kube events when a container has restarted - the reason for the previous container death is consulted to see if it was OOM-killed.

Instead, require Prometheus and kube-state-metrics, and use that to obtain OOM-kill information via kube-state-metrics?

* We still may not get the level of completeness we would on our own (controller watching events and in-cluster changes).

#### Help Differentiate Custom Resource Pod-controllers in Action Items

Here, a "custom resource pod-controller" means a CRD that manages pods, whether directly or via a known pod-controller like a Deployment. We want to provide enough useful information in an action item to help customers identify the correct area of a custom resource manifest which may manage multiple pod-controllers (Deployments or other intermediary CRDs).

For standard pod-controllers (deployments et al) the current action-item reflecting namespace, resource name, and container name, is enough to differentiate a workload for customers. In the case of CRDs, a higher-level pod-controller may have more specific details that we can't plan for / know about, that the customer may need to differentiate I.E. the zone for a single managed database CRD.

Likely including Pod labels in the action item, will provide enough useful information. We may be able to reliably understand/modify memory limits of some CRDs, but the complexity of some is not worth our risk.


For an example CRE situation, the [Vitess database operator](https://vitess.io/docs/get-started/operator/) has a `VitessCluster` CRD that manages multiple groups of Pods using multiple Deployments and a `EtcdLockserver` CRD.

* The Pod labels include enough background for the customer to map an OOM-kill to the area of the parent `VitessCluster` resource that the customer should adjust.
* Examples of information available in pod labels include the Vitess cluster name, cell/zone, and component (etcd, vtctld, vtgateway).
* Within the owning `VitessCluster` pod-controller, limits can be set in a variety of places that inform the child managed pod-controller resources. For example:
	* spec -> partitionings -> {partition equality list} -> shardTemplate -> tabletPools -> cell -> mysqld
	* spec -> cells -> {cell name} -> gateway -> vttablet

### Controller Code

* Move CLI flag validation into package constructors that create a new controller or ReportBuilder?
	* I.E. validating input like updateMemoryLimitsIncrement should be less than updateMemoryLimitsMax. Currently this happens in CLI / flags code, but could happen in package `With*` functional-options code, which would pass errors back up to CLI processing.
* Reflect package configuration defaults as CLI options?
	* Package constructors set defaults, which could be fetched and exposed in CLI flag definitions. To do this, `main.go` might fetch a default controller and ReportBuilder configuration to get its default values, for use as flag defaults.

### General Code Layout

* Move code from `src` to `pkg` sub-directory?
* Minimize `cmd/main.go` by moving more code into packages?
* Our functions in `controller.go` are still pretty long, could use more refactoring; breaking down into smaller functions. THere are too many if/else statements, included to log "else" cases like when we don't update memory limits for a variety of reasons (feature not enabled, not enough OOM-kills, namespace not allowed).
* Replace `COntext.TODO()` with another context? What (controller vs. report operations) should share a context?
* Where/how to give credit, that we’ve used OOM Event Generator project code in our plugin??? https://github.com/xing/kubernetes-oom-event-generator
