package watcher

import (
	"time"

	"github.com/FairwindsOps/insights-plugins/realtime-reporter/pkg/handlers"
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

	informer.AddEventHandler(handlers.PolarisHandler)

	return &Watcher{client, informer}, nil
}

type Watcher struct {
	client   *Client
	informer Informer
}

func (w *Watcher) Run(stopCh chan struct{}) {
	w.informer.Start(stopCh)
}
