package util

import (
	"os"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	listersV1 "k8s.io/client-go/listers/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" // needed for local development with .kube/config
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// Clientset abstracts the cluster config loading both locally and on Kubernetes
func Clientset() (kubernetes.Interface, dynamic.Interface, meta.RESTMapper) {
	// Try to load in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		glog.V(3).Infof("Could not load in-cluster config: %v", err)

		// Fall back to local config
		config, err = clientcmd.BuildConfigFromFlags("", os.Getenv("HOME")+"/.kube/config")
		if err != nil {
			glog.Fatalf("Failed to load client config: %v", err)
		}
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		glog.Fatalf("Failed to create kubernetes client: %v", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		glog.Fatalf("Error creating dynamic kubernetes client: %v", err)
	}
	RESTMapper, err := apiutil.NewDynamicRESTMapper(config)
	if err != nil {
		glog.Fatalf("Error creating REST Mapper: %v", err)
	}

	return client, dynamicClient, RESTMapper
}

type PodLister interface {
	Pods(namespace string) listersV1.PodNamespaceLister
}
