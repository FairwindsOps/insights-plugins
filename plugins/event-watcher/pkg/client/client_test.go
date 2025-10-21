package client

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// mockRESTMapper is a mock implementation of meta.RESTMapper for testing
type mockRESTMapper struct {
	resourceMap map[string]schema.GroupVersionResource
}

func (m *mockRESTMapper) KindFor(resource schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, nil
}

func (m *mockRESTMapper) KindsFor(resource schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	return nil, nil
}

func (m *mockRESTMapper) ResourceFor(input schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	if gvr, exists := m.resourceMap[input.Resource]; exists {
		return gvr, nil
	}
	return schema.GroupVersionResource{}, &meta.NoResourceMatchError{}
}

func (m *mockRESTMapper) ResourcesFor(input schema.GroupVersionResource) ([]schema.GroupVersionResource, error) {
	return nil, nil
}

func (m *mockRESTMapper) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	return nil, nil
}

func (m *mockRESTMapper) RESTMappings(gk schema.GroupKind, versions ...string) ([]*meta.RESTMapping, error) {
	return nil, nil
}

func (m *mockRESTMapper) ResourceSingularizer(resource string) (singular string, err error) {
	return "", nil
}
