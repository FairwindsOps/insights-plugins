package kube

import (
	"context"
	"sync"

	"github.com/sirupsen/logrus"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	ctrl "sigs.k8s.io/controller-runtime"
)

var once sync.Once
var singletonClient *Client

type Client struct {
	RestMapper       meta.RESTMapper
	DynamicInterface dynamic.Interface
}

func GetKubeClient() *Client {
	once.Do(func() {
		if singletonClient == nil {
			var err error
			singletonClient, err = getKubeClient()
			if err != nil {
				logrus.Errorf("Error retrieving kubernetes client: %v", err)
				singletonClient = nil
			}
		}
	})
	return singletonClient
}

func getKubeClient() (*Client, error) {
	config, err := ctrl.GetConfig()
	if err != nil {
		return nil, err
	}
	dynamicInterface, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	kube, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	groupResources, err := restmapper.GetAPIGroupResources(kube.Discovery())
	if err != nil {
		return nil, err
	}
	restMapper := restmapper.NewDiscoveryRESTMapper(groupResources)

	client := Client{
		restMapper,
		dynamicInterface,
	}
	return &client, nil
}

func (client Client) GetData(ctx context.Context, group, kind string) ([]interface{}, error) {
	mapping, err := client.RestMapper.RESTMapping(schema.GroupKind{Group: group, Kind: kind})
	if err != nil {
		return nil, err
	}
	gvr := mapping.Resource
	list, err := client.DynamicInterface.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	items := make([]interface{}, 0)
	for _, item := range list.Items {
		items = append(items, item.Object)
	}
	return items, nil
}
