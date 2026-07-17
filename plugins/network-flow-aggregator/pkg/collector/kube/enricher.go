package kube

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	appslisters "k8s.io/client-go/listers/apps/v1"
	batchlisters "k8s.io/client-go/listers/batch/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	discoverylisters "k8s.io/client-go/listers/discovery/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	defaultResync     = 10 * time.Minute
	maxOwnerWalkDepth = 8
)

type WorkloadIdentity struct {
	Namespace string
	Kind      string
	Name      string
}

type DstIdentity struct {
	Namespace string
	Kind      string
	Name      string
	Addr      string
}

type Enricher struct {
	log            *slog.Logger
	podLister      corelisters.PodLister
	svcLister      corelisters.ServiceLister
	epSliceLister  discoverylisters.EndpointSliceLister
	rsLister       appslisters.ReplicaSetLister
	jobLister      batchlisters.JobLister
	podsSynced     cache.InformerSynced
	svcsSynced     cache.InformerSynced
	epSlicesSynced cache.InformerSynced
	rsSynced       cache.InformerSynced
	deploySynced   cache.InformerSynced
	stsSynced      cache.InformerSynced
	dsSynced       cache.InformerSynced
	jobSynced      cache.InformerSynced
	cronJobSynced  cache.InformerSynced
}

func NewEnricher(ctx context.Context, clients *Clients, log *slog.Logger) (*Enricher, error) {
	if log == nil {
		log = slog.Default()
	}

	factory := informers.NewSharedInformerFactory(clients.Kubernetes, defaultResync)
	podInformer := factory.Core().V1().Pods()
	svcInformer := factory.Core().V1().Services()
	epSliceInformer := factory.Discovery().V1().EndpointSlices()
	rsInformer := factory.Apps().V1().ReplicaSets()
	deployInformer := factory.Apps().V1().Deployments()
	stsInformer := factory.Apps().V1().StatefulSets()
	dsInformer := factory.Apps().V1().DaemonSets()
	jobInformer := factory.Batch().V1().Jobs()
	cronJobInformer := factory.Batch().V1().CronJobs()

	e := &Enricher{
		log:            log,
		podLister:      podInformer.Lister(),
		svcLister:      svcInformer.Lister(),
		epSliceLister:  epSliceInformer.Lister(),
		rsLister:       rsInformer.Lister(),
		jobLister:      jobInformer.Lister(),
		podsSynced:     podInformer.Informer().HasSynced,
		svcsSynced:     svcInformer.Informer().HasSynced,
		epSlicesSynced: epSliceInformer.Informer().HasSynced,
		rsSynced:       rsInformer.Informer().HasSynced,
		deploySynced:   deployInformer.Informer().HasSynced,
		stsSynced:      stsInformer.Informer().HasSynced,
		dsSynced:       dsInformer.Informer().HasSynced,
		jobSynced:      jobInformer.Informer().HasSynced,
		cronJobSynced:  cronJobInformer.Informer().HasSynced,
	}

	factory.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(),
		e.podsSynced, e.svcsSynced, e.epSlicesSynced,
		e.rsSynced, e.deploySynced, e.stsSynced, e.dsSynced,
		e.jobSynced, e.cronJobSynced,
	) {
		return nil, fmt.Errorf("informer cache sync")
	}

	log.Info("kubernetes informers synced")
	return e, nil
}

func (e *Enricher) ResolveSrcWorkload(namespace, podName string) WorkloadIdentity {
	fallback := WorkloadIdentity{Namespace: namespace, Kind: "Pod", Name: podName}
	if namespace == "" || podName == "" {
		return fallback
	}

	pod, err := e.podLister.Pods(namespace).Get(podName)
	if err != nil {
		e.log.Debug("pod lookup failed", "namespace", namespace, "pod", podName, "err", err)
		return fallback
	}

	return e.resolveTopWorkload(pod)
}

func (e *Enricher) resolveTopWorkload(pod *corev1.Pod) WorkloadIdentity {
	ns := pod.Namespace
	kind := "Pod"
	name := pod.Name
	owners := pod.OwnerReferences

	for depth := 0; depth < maxOwnerWalkDepth; depth++ {
		owner := controllerOwner(owners)
		if owner == nil {
			break
		}
		if owner.Kind == "Node" {
			// Static pods are owned by the Node; keep the Pod identity.
			break
		}

		kind = owner.Kind
		name = owner.Name

		switch owner.Kind {
		case "ReplicaSet":
			rs, err := e.rsLister.ReplicaSets(ns).Get(owner.Name)
			if err != nil {
				return WorkloadIdentity{Namespace: ns, Kind: kind, Name: name}
			}
			owners = rs.OwnerReferences
		case "Job":
			job, err := e.jobLister.Jobs(ns).Get(owner.Name)
			if err != nil {
				return WorkloadIdentity{Namespace: ns, Kind: kind, Name: name}
			}
			owners = job.OwnerReferences
		default:
			// Deployment, StatefulSet, DaemonSet, CronJob, and unknown controllers.
			return WorkloadIdentity{Namespace: ns, Kind: kind, Name: name}
		}
	}

	return WorkloadIdentity{Namespace: ns, Kind: kind, Name: name}
}

func controllerOwner(owners []metav1.OwnerReference) *metav1.OwnerReference {
	for i := range owners {
		if owners[i].Controller != nil && *owners[i].Controller {
			return &owners[i]
		}
	}
	if len(owners) > 0 {
		return &owners[0]
	}
	return nil
}

func (e *Enricher) ResolveDst(addr string, port uint32) DstIdentity {
	fallback := DstIdentity{Addr: addr}
	if addr == "" {
		return fallback
	}

	services, slices, ok := e.listServicesAndSlices()
	if !ok {
		return fallback
	}

	idx := buildDstIndex(services, slices)
	if ref, ok := idx.lookup(addr, port); ok {
		return DstIdentity{
			Namespace: ref.Namespace,
			Kind:      "Service",
			Name:      ref.Name,
			Addr:      addr,
		}
	}

	if dst, ok := e.resolveDstFromPodIP(addr); ok {
		return dst
	}

	return fallback
}

func (e *Enricher) resolveDstFromPodIP(addr string) (DstIdentity, bool) {
	pods, ok := e.listPods()
	if !ok {
		return DstIdentity{}, false
	}
	ref, ok := buildPodIPIndex(pods).lookup(addr)
	if !ok {
		return DstIdentity{}, false
	}
	wl := e.ResolveSrcWorkload(ref.Namespace, ref.Name)
	return DstIdentity{
		Namespace: wl.Namespace,
		Kind:      wl.Kind,
		Name:      wl.Name,
		Addr:      addr,
	}, true
}

func (e *Enricher) LookupEndpoint(addr string, port uint32) (EndpointEntry, bool) {
	if addr == "" || port == 0 {
		return EndpointEntry{}, false
	}
	services, slices, ok := e.listServicesAndSlices()
	if !ok {
		return EndpointEntry{}, false
	}
	idx := buildEndpointIndex(services, slices)
	return idx.lookup(addr, port)
}

func (e *Enricher) ResolveBackendFromEndpoint(addr string, port uint32) (BackendIdentity, bool) {
	entry, ok := e.LookupEndpoint(addr, port)
	if !ok || entry.PodName == "" {
		return BackendIdentity{}, false
	}
	workload := e.ResolveSrcWorkload(entry.PodNamespace, entry.PodName)
	return BackendIdentity{
		PodNamespace:      entry.PodNamespace,
		PodName:           entry.PodName,
		WorkloadNamespace: workload.Namespace,
		WorkloadKind:      workload.Kind,
		WorkloadName:      workload.Name,
		ServiceNamespace:  entry.ServiceNamespace,
		ServiceName:       entry.ServiceName,
	}, true
}

func (e *Enricher) listPods() ([]*corev1.Pod, bool) {
	pods, err := e.podLister.List(labels.Everything())
	if err != nil {
		e.log.Debug("pod list failed", "err", err)
		return nil, false
	}
	return pods, true
}

func (e *Enricher) listServicesAndSlices() ([]*corev1.Service, []*discoveryv1.EndpointSlice, bool) {
	services, err := e.svcLister.List(labels.Everything())
	if err != nil {
		e.log.Debug("service list failed", "err", err)
		return nil, nil, false
	}
	slices, err := e.epSliceLister.List(labels.Everything())
	if err != nil {
		e.log.Debug("endpointslice list failed", "err", err)
		return nil, nil, false
	}
	return services, slices, true
}
