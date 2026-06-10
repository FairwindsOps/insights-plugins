package kube

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"

	ctrlclient "github.com/fairwindsops/controller-utils/pkg/controller"
)

type Clients struct {
	Kubernetes kubernetes.Interface
	Dynamic    dynamic.Interface
	RESTMapper meta.RESTMapper
	Controller ctrlclient.Client
}

func NewClients(ctx context.Context, kubeconfig string) (*Clients, error) {
	cfg, err := restConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	kube, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("kubernetes client: %w", err)
	}

	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("dynamic client: %w", err)
	}

	resources, err := restmapper.GetAPIGroupResources(kube.Discovery())
	if err != nil {
		return nil, fmt.Errorf("discovery: %w", err)
	}
	restMapper := restmapper.NewDiscoveryRESTMapper(resources)

	return &Clients{
		Kubernetes: kube,
		Dynamic:    dyn,
		RESTMapper: restMapper,
		Controller: ctrlclient.Client{
			Context:    ctx,
			Dynamic:    dyn,
			RESTMapper: restMapper,
		},
	}, nil
}

func restConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("kubeconfig: %w", err)
		}
		return cfg, nil
	}

	cfg, err := rest.InClusterConfig()
	if err == nil {
		return cfg, nil
	}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	cfg, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules,
		&clientcmd.ConfigOverrides{},
	).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("kubernetes config: %w", err)
	}
	return cfg, nil
}
