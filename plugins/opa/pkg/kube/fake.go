package kube

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicFake "k8s.io/client-go/dynamic/fake"
)

var builtInGroupVersions = []schema.GroupVersion{
	{
		Group: "apps",
		Version: "v1",
	},
	{
		Group: "autoscaling",
		Version: "v1",
	},
	{
		Group: "autoscaling",
		Version: "v2beta1",
	},
}

var builtInKinds = []schema.GroupVersionKind{
	{
		Group:   "apps",
		Version: "v1",
		Kind:    "ReplicaSet",
	},
	{
		Group:   "autoscaling",
		Version: "v1",
		Kind:    "HorizontalPodAutoscaler",
	},	
	{
		Group:   "autoscaling",
		Version: "v2beta1",
		Kind:    "HorizontalPodAutoscaler",
	},
	{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	},
}

func SetFakeClient() *Client {
	scheme := k8sruntime.NewScheme()
	for _, gvk := range builtInKinds {
		listKind := schema.GroupVersionKind{
			Group: gvk.Group,
			Version: gvk.Version,
			Kind: gvk.Kind + "List",
		}
		scheme.AddKnownTypeWithName(listKind,  &unstructured.UnstructuredList{})
	}

	dynamic := dynamicFake.NewSimpleDynamicClient(scheme)
	restMapper := meta.NewDefaultRESTMapper(builtInGroupVersions)
	for _, gvk := range builtInKinds {
		restMapper.Add(gvk, meta.RESTScopeNamespace)
	}

	client := Client{
		restMapper,
		dynamic,
	}
	singletonClient = &client
	return singletonClient
}

func AddFakeDeployment() {
	client := GetKubeClient()
	mapping, err := client.RestMapper.RESTMapping(schema.GroupKind{Group: "apps", Kind: "Deployment"})
	if err != nil {
		panic(err)
	}
	gvr := mapping.Resource
	_, err = client.DynamicInterface.Resource(gvr).Namespace("test").Create(context.TODO(), &unstructured.Unstructured{}, metav1.CreateOptions{})
	if err != nil {
		panic(err)
	}
}
