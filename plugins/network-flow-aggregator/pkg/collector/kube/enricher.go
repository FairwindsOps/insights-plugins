package kube

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	corelisters "k8s.io/client-go/listers/core/v1"
	discoverylisters "k8s.io/client-go/listers/discovery/v1"
	"k8s.io/client-go/tools/cache"

	ctrlclient "github.com/fairwindsops/controller-utils/pkg/controller"
)

const defaultResync = 10 * time.Minute

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
	log             *slog.Logger
	controller      ctrlclient.Client
	podLister       corelisters.PodLister
	svcLister       corelisters.ServiceLister
	epSliceLister   discoverylisters.EndpointSliceLister
	podsSynced      cache.InformerSynced
	svcsSynced      cache.InformerSynced
	epSlicesSynced  cache.InformerSynced
	ownerCache      map[string]unstructured.Unstructured
	ownerMu         sync.Mutex
}

func NewEnricher(ctx context.Context, clients *Clients, log *slog.Logger) (*Enricher, error) {
	if log == nil {
		log = slog.Default()
	}

	factory := informers.NewSharedInformerFactory(clients.Kubernetes, defaultResync)
	podInformer := factory.Core().V1().Pods()
	svcInformer := factory.Core().V1().Services()
	epSliceInformer := factory.Discovery().V1().EndpointSlices()

	e := &Enricher{
		log:            log,
		controller:     clients.Controller,
		podLister:      podInformer.Lister(),
		svcLister:      svcInformer.Lister(),
		epSliceLister:  epSliceInformer.Lister(),
		podsSynced:     podInformer.Informer().HasSynced,
		svcsSynced:     svcInformer.Informer().HasSynced,
		epSlicesSynced: epSliceInformer.Informer().HasSynced,
		ownerCache:     make(map[string]unstructured.Unstructured),
	}

	factory.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), e.podsSynced, e.svcsSynced, e.epSlicesSynced) {
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

	u, err := podToUnstructured(pod)
	if err != nil {
		e.log.Debug("pod conversion failed", "namespace", namespace, "pod", podName, "err", err)
		return fallback
	}

	e.ownerMu.Lock()
	top, err := e.controller.GetTopController(u, e.ownerCache)
	e.ownerMu.Unlock()
	if err != nil {
		e.log.Debug("top controller lookup failed", "namespace", namespace, "pod", podName, "err", err)
		return fallback
	}

	return workloadIdentityFromController(top, namespace, podName)
}

func workloadIdentityFromController(top unstructured.Unstructured, namespace, podName string) WorkloadIdentity {
	kind := top.GetKind()
	name := top.GetName()
	ns := top.GetNamespace()
	if ns == "" {
		ns = namespace
	}
	if kind == "" {
		kind = "Pod"
	}
	if name == "" {
		name = podName
	}
	return WorkloadIdentity{
		Namespace: ns,
		Kind:      kind,
		Name:      name,
	}
}

func (e *Enricher) ResolveDst(addr string, port uint32) DstIdentity {
	fallback := DstIdentity{Addr: addr}
	if addr == "" {
		return fallback
	}

	services, err := e.svcLister.List(labels.Everything())
	if err != nil {
		e.log.Debug("service list failed", "err", err)
		return fallback
	}

	slices, err := e.epSliceLister.List(labels.Everything())
	if err != nil {
		e.log.Debug("endpointslice list failed", "err", err)
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
	return fallback
}

func podToUnstructured(pod *corev1.Pod) (unstructured.Unstructured, error) {
	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pod)
	if err != nil {
		return unstructured.Unstructured{}, err
	}
	return unstructured.Unstructured{Object: m}, nil
}
