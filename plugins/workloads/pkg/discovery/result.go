package discovery

// Result contains container images discovered for repository inventory.
type Result struct {
	Images []ImageResult
}

// ImageResult is a container image with workload owners.
type ImageResult struct {
	Name    string
	ID      string
	PullRef string
	Owners  []OwnerResult
}

// OwnerResult identifies a Kubernetes workload that runs an image.
type OwnerResult struct {
	Namespace string
	Kind      string
	Name      string
	Container string
	// Labels/Annotations/PodLabels/PodAnnotations are set only for supplemental owners
	// (orphan Pod, standalone Job) that do not appear in Controllers[]. CronJob owners
	// omit these maps even when discovered via a Job pod.
	Labels         map[string]string `json:",omitempty"`
	Annotations    map[string]string `json:",omitempty"`
	PodLabels      map[string]string `json:",omitempty"`
	PodAnnotations map[string]string `json:",omitempty"`
}
