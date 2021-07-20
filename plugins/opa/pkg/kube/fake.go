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

func SetFakeClient() *Client {
	scheme := k8sruntime.NewScheme()
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "ReplicaSetList",
	}, &unstructured.UnstructuredList{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{
		Group:   "autoscaling",
		Version: "v1",
		Kind:    "HorizontalPodAutoscalerList",
	}, &unstructured.UnstructuredList{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "DeploymentList",
	}, &unstructured.UnstructuredList{})
	dynamic := dynamicFake.NewSimpleDynamicClient(scheme)
	gv := schema.GroupVersion{Group: "apps", Version: "v1"}
	gv2 := schema.GroupVersion{Group: "autoscaling", Version: "v1"}
	gvk := gv.WithKind("Deployment")
	restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{gv, gv2})
	restMapper.Add(gvk, meta.RESTScopeNamespace)
	gvk = gv2.WithKind("HorizontalPodAutoscaler")
	restMapper.Add(gvk, meta.RESTScopeNamespace)
	gvk = gv.WithKind("ReplicaSet")
	restMapper.Add(gvk, meta.RESTScopeNamespace)
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
