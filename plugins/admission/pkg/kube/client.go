package kube

import (
	"context"
	"sync"

	"github.com/sirupsen/logrus"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

func (client Client) GetObject(ctx context.Context, namespace, kind, version, name string, dynamicClient dynamic.Interface, restMapper meta.RESTMapper) (*unstructured.Unstructured, error) {
	fqKind := schema.FromAPIVersionAndKind(version, kind)
	mapping, err := restMapper.RESTMapping(fqKind.GroupKind(), fqKind.Version)
	if err != nil {
		return nil, err
	}
	object, err := dynamicClient.Resource(mapping.Resource).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	return object, err
}
