package kube

import (
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	dynamicFake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/restmapper"
)

func SetFakeClient() *Client {
	objects := []k8sruntime.Object{}
	kube := k8sfake.NewSimpleClientset(objects...)
	dynamic := dynamicFake.NewSimpleDynamicClient(k8sruntime.NewScheme())
	groupResources, err := restmapper.GetAPIGroupResources(kube.Discovery())
	if err != nil {
		panic(err)
	}
	restMapper := restmapper.NewDiscoveryRESTMapper(groupResources)
	client := Client{
		restMapper,
		dynamic,
	}
	singletonClient = &client
	return singletonClient
}
