package watcher

import (
	"time"

	"github.com/FairwindsOps/insights-plugins/realtime-reporter/pkg/handlers"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

type Informer interface {
	AddEventHandler(handler handlers.EventHandler)
	HasSynced() bool
	Start(ch <-chan struct{})
}

type NewInformerFunc func(client *Client) (*multiResourceInformer, error)

func NewMultiResourceInformer(resyncPeriod time.Duration) NewInformerFunc {
	return func(client *Client) (*multiResourceInformer, error) {
		informers := make(map[string]map[string]cache.SharedIndexInformer)

		resources := make(map[string]schema.GroupVersionResource)
		for _, r := range viper.GetStringSlice("resources") {
			gvr, err := getGVRFromResource(client.discoveryMapper, r)
			if err != nil {
				return nil, err
			}
			resources[r] = gvr
		}

		dynamicInformers := make([]dynamicinformer.DynamicSharedInformerFactory, 0, len(viper.GetStringSlice("namespaces")))

		for _, ns := range viper.GetStringSlice("namespaces") {

			namespace := getNamespace(ns)
			di := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
				client.dynamicClient,
				resyncPeriod,
				namespace,
				nil,
			)

			for r, gvr := range resources {
				if _, ok := informers[ns]; !ok {
					informers[ns] = make(map[string]cache.SharedIndexInformer)
				}
				informers[ns][r] = di.ForResource(gvr).Informer()
			}

			dynamicInformers = append(dynamicInformers, di)
		}

		return &multiResourceInformer{
			resourceToGVR:      resources,
			resourceToInformer: informers,
			informerFactory:    dynamicInformers,
		}, nil
	}
}

type multiResourceInformer struct {
	resourceToGVR      map[string]schema.GroupVersionResource
	resourceToInformer map[string]map[string]cache.SharedIndexInformer
	informerFactory    []dynamicinformer.DynamicSharedInformerFactory
}

var _ Informer = &multiResourceInformer{}

// AddEventHandler adds the handler to each namespaced informer
func (i *multiResourceInformer) AddEventHandler(handler handlers.EventHandler) {
	for _, ki := range i.resourceToInformer {
		for kind, informer := range ki {
			informer.AddEventHandler(handler(kind))
		}
	}
}

// HasSynced checks if each namespaced informer has synced
func (i *multiResourceInformer) HasSynced() bool {
	for _, ki := range i.resourceToInformer {
		for _, informer := range ki {
			if ok := informer.HasSynced(); !ok {
				return ok
			}
		}
	}

	return true
}

func (i *multiResourceInformer) Start(stopCh <-chan struct{}) {
	for _, informer := range i.informerFactory {
		informer.Start(stopCh)
	}
}
