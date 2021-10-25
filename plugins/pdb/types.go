package main

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type Hpa struct {
	Kind        string `json:"kind"`
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	MinReplicas int    `json:"min_replicas"`
	MaxReplicas int    `json:"max_replicas"`
}

type Pdb struct {
	Kind           string  `json:"kind"`
	Namespace      string  `json:"namespace"`
	Name           string  `json:"name"`
	MinAvailable   float64 `json:"min_available"`
	MaxUnavailable float64 `json:"max_unavailable"`
	DesiredHealthy int32   `json:"desired_healthy"`
}

type Workload struct {
	Kind            string                  `json:"kind"`
	ApiVersion      string                  `json:"api_version"`
	Namespace       string                  `json:"namespace"`
	Name            string                  `json:"name"`
	Replicas        int32                   `json:"replicas"`
	Labels          map[string]string       `json:"labels"`
	OwnerReferences []metav1.OwnerReference `json:"owner_references"`
}

type ProblemWorkload struct {
	Hpa            Hpa      `json:"hpa"`
	Pdb            Pdb      `json:"pdb"`
	Workload       Workload `json:"workload"`
	Recommendation string   `json:"recommendation"`
}
