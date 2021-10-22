package main

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type Hpa struct {
	Kind        string
	Namespace   string
	Name        string
	MinReplicas int
	MaxReplicas int
}

type Pdb struct {
	Kind           string
	Namespace      string
	Name           string
	MinAvailable   float64
	MaxUnavailable float64
	DesiredHealthy int32
}

type Workload struct {
	Kind            string
	ApiVersion      string
	Namespace       string
	Name            string
	Replicas        int32
	Labels          map[string]string
	OwnerReferences []metav1.OwnerReference
}

type ProblemWorkload struct {
	Hpa            Hpa
	Pdb            Pdb
	Workload       Workload
	Recommendation string
}
