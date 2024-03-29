package watcher

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/viper"

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

	fmt.Println(viper.GetBool("polaris-enabled"))
	fmt.Println(viper.GetString("polaris-config"))

	if viper.GetBool("polaris-enabled") {
		if viper.GetString("polaris-config") == "" {
			return nil, errors.New("polaris configuration file path must be provided when Polaris is enabled")
		}
		informer.AddEventHandler(handlers.PolarisHandler)
	} else {
		return nil, errors.New("no valid handler has been specified")
	}

	return &Watcher{client, informer}, nil
}

type Watcher struct {
	client   *Client
	informer Informer
}

func (w *Watcher) Run(stopCh chan struct{}) {
	w.informer.Start(stopCh)
}
