package models

import "strings"

// Resource identifies a Kubernetes owner for an image.
type Resource struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Container string `json:"container,omitempty"`
}

// DiscoveredImage is an image observed running in the cluster.
type DiscoveredImage struct {
	Name    string     `json:"name"`
	ID      string     `json:"id"`
	PullRef string     `json:"pullRef"`
	Owners  []Resource `json:"owners"`
}

// Digest returns the digest portion of the image ID when present.
func (i DiscoveredImage) Digest() string {
	if i.ID == "" {
		return ""
	}
	parts := strings.Split(i.ID, "@")
	if len(parts) > 1 {
		return parts[1]
	}
	return i.ID
}

// UniqueKey returns a stable deduplication key for the image.
func (i DiscoveredImage) UniqueKey() string {
	if i.ID != "" {
		return i.ID
	}
	return i.Name
}
