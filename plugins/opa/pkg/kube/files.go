package kube

import (
	"context"
	"encoding/json"
	"strings"

	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicFake "k8s.io/client-go/dynamic/fake"
)

// SetFileClient sets the singletonClient to be a static client based on the values passed in.
func SetFileClient(objects []map[string]interface{}) *Client {
	groupVersionsFound := map[string]bool{}
	var groupVersions []schema.GroupVersion
	for _, obj := range objects {
		apiVersion := obj["apiVersion"].(string)
		if groupVersionsFound[apiVersion] {
			continue
		}
		versionSplit := strings.Split(apiVersion, "/")
		var version string
		var group string
		if len(versionSplit) > 1 {
			version = versionSplit[1]
			group = versionSplit[0]
		} else {
			version = versionSplit[0]
		}
		groupVersionsFound[apiVersion] = true
		groupVersions = append(groupVersions, schema.GroupVersion{Group: group, Version: version})

	}
	gvks := []schema.GroupVersionKind{}
	for _, obj := range objects {
		apiVersion := obj["apiVersion"].(string)
		versionSplit := strings.Split(apiVersion, "/")
		var version string
		var group string
		if len(versionSplit) > 1 {
			version = versionSplit[1]
			group = versionSplit[0]
		} else {
			version = versionSplit[0]
		}
		gv := schema.GroupVersion{Group: group, Version: version}
		gvk := gv.WithKind(obj["kind"].(string))
		gvks = append(gvks, gvk)
	}
	
	scheme := k8sruntime.NewScheme()
	
	for _, gvk := range gvks {
		gvkList := schema.GroupVersionKind{
			Group: gvk.Group,
			Version: gvk.Version,
			Kind: gvk.Kind + "List",
		}
		scheme.AddKnownTypeWithName(gvkList, &unstructured.UnstructuredList{})
	}
	dynamic := dynamicFake.NewSimpleDynamicClient(scheme)
	restMapper := meta.NewDefaultRESTMapper(groupVersions)
	
	for _, gvk := range gvks {
		restMapper.Add(gvk, meta.RESTScopeNamespace)

		gk := schema.GroupKind{
			Group: gvk.Group,
			Kind: gvk.Kind,
		}
		mapping, err := restMapper.RESTMapping(gk)
		if err != nil {
			panic(err)
		}
		gvr := mapping.Resource
		// Marshal then Unmarshal to deep copy directly into an unstructured object
		objBytes, err := json.Marshal(obj)
		if err != nil {
			panic(err)
		}

		var unstructure unstructured.Unstructured
		err = unstructure.UnmarshalJSON(objBytes)
		if err != nil {
			panic(err)
		}
		namespace := unstructure.GetNamespace()
		if namespace == "" {
			namespace = "objects"
		}
		_, err = dynamic.Resource(gvr).Namespace(namespace).Create(context.Background(), &unstructure, metav1.CreateOptions{})
		if err != nil {
			statusError, ok := err.(*kubeerrors.StatusError)
			if !ok || statusError.ErrStatus.Reason != metav1.StatusReasonAlreadyExists {
				panic(err)
			}
		}
	}
	client := Client{
		restMapper,
		dynamic,
	}
	singletonClient = &client
	return singletonClient
}
