package kube

import (
	"log/slog"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appslisters "k8s.io/client-go/listers/apps/v1"
	batchlisters "k8s.io/client-go/listers/batch/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

func boolPtr(v bool) *bool { return &v }

func TestResolveSrcWorkloadDeployment(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "payments-7d9c8b6f5-abcde",
			Namespace: "prod",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "apps/v1",
				Kind:       "ReplicaSet",
				Name:       "payments-7d9c8b6f5",
				Controller: boolPtr(true),
			}},
		},
	}
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "payments-7d9c8b6f5",
			Namespace: "prod",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "payments",
				Controller: boolPtr(true),
			}},
		},
	}

	e := newWorkloadTestEnricher(t, []*corev1.Pod{pod}, []*appsv1.ReplicaSet{rs}, nil)
	id := e.ResolveSrcWorkload("prod", "payments-7d9c8b6f5-abcde")
	if id.Kind != "Deployment" || id.Name != "payments" || id.Namespace != "prod" {
		t.Fatalf("identity = %#v", id)
	}
}

func TestResolveSrcWorkloadCronJob(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backup-29123456-xyz",
			Namespace: "ops",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "batch/v1",
				Kind:       "Job",
				Name:       "backup-29123456",
				Controller: boolPtr(true),
			}},
		},
	}
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backup-29123456",
			Namespace: "ops",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "batch/v1",
				Kind:       "CronJob",
				Name:       "backup",
				Controller: boolPtr(true),
			}},
		},
	}

	e := newWorkloadTestEnricher(t, []*corev1.Pod{pod}, nil, []*batchv1.Job{job})
	id := e.ResolveSrcWorkload("ops", "backup-29123456-xyz")
	if id.Kind != "CronJob" || id.Name != "backup" {
		t.Fatalf("identity = %#v", id)
	}
}

func TestResolveSrcWorkloadStaticPod(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-scheduler-node-1",
			Namespace: "kube-system",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "v1",
				Kind:       "Node",
				Name:       "node-1",
				Controller: boolPtr(true),
			}},
		},
	}

	e := newWorkloadTestEnricher(t, []*corev1.Pod{pod}, nil, nil)
	id := e.ResolveSrcWorkload("kube-system", "kube-scheduler-node-1")
	if id.Kind != "Pod" || id.Name != "kube-scheduler-node-1" {
		t.Fatalf("identity = %#v", id)
	}
}

func TestResolveSrcWorkloadMissingPod(t *testing.T) {
	e := newWorkloadTestEnricher(t, nil, nil, nil)
	id := e.ResolveSrcWorkload("default", "missing")
	if id.Kind != "Pod" || id.Name != "missing" {
		t.Fatalf("identity = %#v", id)
	}
}

func newWorkloadTestEnricher(t *testing.T, pods []*corev1.Pod, rss []*appsv1.ReplicaSet, jobs []*batchv1.Job) *Enricher {
	t.Helper()

	podIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	for _, pod := range pods {
		if err := podIndexer.Add(pod); err != nil {
			t.Fatalf("add pod: %v", err)
		}
	}
	rsIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	for _, rs := range rss {
		if err := rsIndexer.Add(rs); err != nil {
			t.Fatalf("add rs: %v", err)
		}
	}
	jobIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	for _, job := range jobs {
		if err := jobIndexer.Add(job); err != nil {
			t.Fatalf("add job: %v", err)
		}
	}

	return &Enricher{
		log:       slog.Default(),
		podLister: corelisters.NewPodLister(podIndexer),
		rsLister:  appslisters.NewReplicaSetLister(rsIndexer),
		jobLister: batchlisters.NewJobLister(jobIndexer),
	}
}
