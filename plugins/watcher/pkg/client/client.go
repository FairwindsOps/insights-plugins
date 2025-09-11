package client

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Client struct {
	RestMapper       meta.RESTMapper
	DynamicInterface dynamic.Interface
	KubeInterface    kubernetes.Interface
}

func NewClient() (*Client, error) {
	config, err := ctrl.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	dynamicInterface, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	kubeInterface, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	groupResources, err := restmapper.GetAPIGroupResources(kubeInterface.Discovery())
	if err != nil {
		return nil, fmt.Errorf("failed to get API group resources: %w", err)
	}

	restMapper := restmapper.NewDiscoveryRESTMapper(groupResources)

	client := &Client{
		RestMapper:       restMapper,
		DynamicInterface: dynamicInterface,
		KubeInterface:    kubeInterface,
	}

	logrus.Info("Successfully created Kubernetes client")
	return client, nil
}

func (c *Client) WatchResources(ctx context.Context, resourceType string) (dynamic.ResourceInterface, error) {
	gvr, err := c.RestMapper.ResourceFor(schema.GroupVersionResource{
		Resource: resourceType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get resource for %s: %w", resourceType, err)
	}

	return c.DynamicInterface.Resource(gvr), nil
}
