package watcher

import (
	"fmt"
	"os"
	"time"

	"github.com/FairwindsOps/insights-plugins/realtime-reporter/pkg/handlers"
	"k8s.io/client-go/tools/cache"
)

const (
	resyncPeriod = time.Duration(1) * time.Minute
)

func NewWatcher() (*Watcher, error) {
	kubeconfig, err := newKubeConfig()
	if err != nil {
		return nil, err
	}

	client, err := newClient(kubeconfig)
	if err != nil {
		return nil, err
	}

	informer, err := NewMultiResourceInformer(resyncPeriod)(client)
	if err != nil {
		return nil, err
	}

	token := os.Getenv("INSIGHTS_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("INSIGHTS_TOKEN environment variable not set")
	}

	informer.AddEventHandler(func(resourceType string) cache.ResourceEventHandlerFuncs {
		return handlers.PolarisHandler(token, resourceType)
	})

	return &Watcher{client, informer}, nil
}

type Watcher struct {
	client   *Client
	informer Informer
}

func (w *Watcher) Run(stopCh chan struct{}) {
	w.informer.Start(stopCh)
}
