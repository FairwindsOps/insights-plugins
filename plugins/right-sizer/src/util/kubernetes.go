package util

import (
	"fmt"
	"os"

	"github.com/golang/glog"

	"gopkg.in/inf.v0"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	listersV1 "k8s.io/client-go/listers/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" // needed for local development with .kube/config
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// KubeClientResources bundles together Kubernetes clients and related
// resources.
type KubeClientResources struct {
	Client        kubernetes.Interface
	DynamicClient dynamic.Interface // used to find owning pod-controller
	RESTMapper    meta.RESTMapper   // used with dynamicClient
}

// Clientset abstracts the cluster config loading both locally and on Kubernetes
func Clientset() KubeClientResources {
	// Try to load in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		glog.V(3).Infof("Could not load in-cluster config, falling back to $KUBECONFIG or ~/.kube/config: %v", err)

		// Fall back to local config
		var kubeConfigFilePath string
		kubeConfigFilePath = os.Getenv("KUBECONFIG")
		if kubeConfigFilePath == "" {
			kubeConfigFilePath = homedir.HomeDir() + "/.kube/config"
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeConfigFilePath)
		if err != nil {
			glog.Fatalf("Failed to load client config %q: %v", kubeConfigFilePath, err)
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

	r := KubeClientResources{
		Client:        client,
		DynamicClient: dynamicClient,
		RESTMapper:    RESTMapper,
	}
	return r
}

type PodLister interface {
	Pods(namespace string) listersV1.PodNamespaceLister
}

// MultiplyResourceQuantity multiplies a
// k8s.io/apimachinery/pkg/api/resource.Quantity with a float64, returning a
// new resource.Quantity.
// If an error is returned, the resource.Quantity will be its zero value.
// To avoid losing precision, the resource.Quantity and multiplier are
// converted to inf.Dec types.
// Note that certain multipliers cause the units of the result to become
// smaller, I.E. from Gi to Mi or Ki.
func MultiplyResourceQuantity(q *resource.Quantity, multiplier float64) (*resource.Quantity, error) {
	// An intermediate type inf.Dec is used, because sometimes converting resource.QUantity to int64 can fail.
	// For reference, see the quantity.AsInt64 documentation: https://pkg.go.dev/k8s.io/apimachinery/pkg/api/resource#Quantity.AsInt64
	qAsDec := q.AsDec()
	multiplierAsDec := new(inf.Dec)
	multiplierAsString := fmt.Sprintf("%.2f", multiplier)
	x, ok := multiplierAsDec.SetString(multiplierAsString)
	if !ok {
		return &resource.Quantity{}, fmt.Errorf("unable to convert multiplier to type inf.Dec, input=%f, stringified=%q, and the result is %v", multiplier, multiplierAsString, x)
	}
	productAsDec := new(inf.Dec)
	productAsDec.Mul(qAsDec, multiplierAsDec)
	product := resource.NewDecimalQuantity(*productAsDec, q.Format) // Retain format from the original quantity.
	return product, nil
}
