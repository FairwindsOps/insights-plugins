package kube

import (
	"context"
	"encoding/json"
	"fmt"
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
	scheme := k8sruntime.NewScheme()
	fmt.Println("register")
	for _, obj := range objects {
		apiVersion := obj["apiVersion"].(string)
		fmt.Println("register api version", apiVersion)
		if groupVersionsFound[apiVersion] {
			continue
		}
		kind := obj["kind"].(string)
		fmt.Println("register kind", apiVersion, kind)
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
		scheme.AddKnownTypeWithName(schema.GroupVersionKind{
			Group:   group,
			Version: version,
			Kind:    kind + "List",
		}, &unstructured.UnstructuredList{})
	}
	dynamic := dynamicFake.NewSimpleDynamicClient(scheme)
	restMapper := meta.NewDefaultRESTMapper(groupVersions)
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
		restMapper.Add(gvk, meta.RESTScopeNamespace)

		mapping, err := restMapper.RESTMapping(schema.GroupKind{Group: group, Kind: obj["kind"].(string)})
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
