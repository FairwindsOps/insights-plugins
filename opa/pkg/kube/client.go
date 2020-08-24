package kube

import (
	"sync"

	meta "k8s.io/apimachinery/pkg/api/meta"
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
	return singletonClient
}

func init() {
	var err error
	singletonClient, err = getKubeClient()
	if err != nil {
		panic(err)
	}
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
