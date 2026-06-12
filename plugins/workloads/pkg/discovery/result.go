package discovery

// Result contains running container images discovered in the cluster.
type Result struct {
	Images []ImageResult
}

// ImageResult is a running container image with workload owners.
type ImageResult struct {
	Name    string
	ID      string
	PullRef string
	Owners  []OwnerResult
}

// OwnerResult identifies a Kubernetes workload that runs an image.
type OwnerResult struct {
	Namespace      string
	Kind           string
	Name           string
	Container      string
	Labels         map[string]string
	Annotations    map[string]string
	PodLabels      map[string]string
	PodAnnotations map[string]string
}
