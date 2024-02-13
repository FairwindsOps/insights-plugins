package watcher

import (
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

type Client struct {
	discoveryClient *discovery.DiscoveryClient
	discoveryMapper *restmapper.DeferredDiscoveryRESTMapper
	dynamicClient   dynamic.Interface
}

func newClient(config *rest.Config) (*Client, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	cacheClient := memory.NewMemCacheClient(discoveryClient)
	cacheClient.Invalidate()

	discoveryMapper := restmapper.NewDeferredDiscoveryRESTMapper(cacheClient)

	return &Client{
		discoveryClient,
		discoveryMapper,
		dynamicClient,
	}, nil
}
