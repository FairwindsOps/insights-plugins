package main

import (
	"context"
	"log"

	autoscalingv1 "k8s.io/api/autoscaling/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func getWorkloads(clientset *kubernetes.Clientset) []Workload {
	workloads := []Workload{} // list of all relevant workloads (deployments, replicasets, statefulsets)

	deployments, err := clientset.AppsV1().Deployments("").List(context.TODO(), metav1.ListOptions{}) // get all deployments
	if err != nil {
		log.Fatalln("error getting deployments:", err) // we don't want a partial report, so fail completely
	} // we have a list of deployments
	for _, wl := range deployments.Items { // we'll go through each deployment and make it a generic "Workload{}" item
		workloads = append(workloads, Workload{ // include deployments in the list of workloads we care about
			Kind:       "Deployment",             // https://github.com/kubernetes/client-go/issues/861
			ApiVersion: "apps/v1",                // same as above^
			Namespace:  wl.ObjectMeta.Namespace,  // KISS
			Name:       wl.ObjectMeta.Name,       // ^
			Replicas:   int32(*wl.Spec.Replicas), // ^
			Labels:     wl.Spec.Template.Labels,  // we will never care about .metadata other than namespace & name because policies (pdb) only effect pods, not the owning object; additionally, autoscalers (hpa) only look at kind, apiversion, name and namespace
		})
	}

	// same as deployments, but for replicasets
	replicasets, err := clientset.AppsV1().ReplicaSets("").List(context.TODO(), metav1.ListOptions{}) // get all replicasets
	if err != nil {
		log.Fatalln("error getting replicasets:", err)
	}
	for _, wl := range replicasets.Items {
		workloads = append(workloads, Workload{ // include replicasets in the list of workloads we care about
			Kind:            "ReplicaSet",
			ApiVersion:      "apps/v1",
			Namespace:       wl.ObjectMeta.Namespace,
			Name:            wl.ObjectMeta.Name,
			Replicas:        int32(*wl.Spec.Replicas),
			Labels:          wl.Spec.Template.Labels,
			OwnerReferences: wl.ObjectMeta.OwnerReferences,
		})
	}

	// same as deployments, but for statefulsets
	statefulsets, err := clientset.AppsV1().StatefulSets("").List(context.TODO(), metav1.ListOptions{}) // get all statefulsets
	if err != nil {
		log.Fatalln("error getting statefulsets:", err)
	}
	for _, wl := range statefulsets.Items {
		workloads = append(workloads, Workload{ // include statefulsets in the list of workloads we care about
			Kind:       "StatefulSet",
			ApiVersion: "apps/v1",
			Namespace:  wl.ObjectMeta.Namespace,
			Name:       wl.ObjectMeta.Name,
			Replicas:   int32(*wl.Spec.Replicas),
			Labels:     wl.Spec.Template.Labels,
		})
	}

	return workloads
}

func getAutoscalers(clientset *kubernetes.Clientset) *autoscalingv1.HorizontalPodAutoscalerList {
	hpas, err := clientset.AutoscalingV1().HorizontalPodAutoscalers("").List(context.TODO(), metav1.ListOptions{}) // get all hpa
	if err != nil {
		log.Fatalln("error getting autoscalers:", err)
	}
	for i := range hpas.Items {
		hpas.Items[i].APIVersion = "autoscaling/v1"
		hpas.Items[i].Kind = "HorizontalPodAutoscaler"
	}

	return hpas
}

func getPdbs(clientset *kubernetes.Clientset) *policyv1.PodDisruptionBudgetList {
	pdbs, err := clientset.PolicyV1().PodDisruptionBudgets("").List(context.TODO(), metav1.ListOptions{}) // get all pdb
	if err != nil {
		log.Fatalln("error getting policies:", err)
	}
	for i := range pdbs.Items {
		pdbs.Items[i].APIVersion = "policy/v1"
		pdbs.Items[i].Kind = "PodDisruptionBudget"
	}

	return pdbs
}
