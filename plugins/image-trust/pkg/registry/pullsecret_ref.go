package registry

// PullSecretRef identifies a kubernetes.io/dockerconfigjson secret referenced by a workload.
type PullSecretRef struct {
	Namespace string
	Name      string
}
