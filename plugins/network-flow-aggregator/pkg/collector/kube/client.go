package kube

import (
	"context"
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Clients struct {
	Kubernetes kubernetes.Interface
}

func NewClients(_ context.Context, kubeconfig string) (*Clients, error) {
	cfg, err := restConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	kube, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("kubernetes client: %w", err)
	}

	return &Clients{Kubernetes: kube}, nil
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
