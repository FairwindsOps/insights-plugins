package util

import (
	"fmt"
	"os"

	"github.com/golang/glog"

	"gopkg.in/inf.v0"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	listersV1 "k8s.io/client-go/listers/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" // needed for local development with .kube/config
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"strings"
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

// findPodSpec searches an unstructured.Unstructured resource for a pod
// specification, returning it as a core.podspec type, along with the path
// where the pod spec was found (such as spec.template.spec).
func FindPodSpec(podController *unstructured.Unstructured) (podSpec *core.PodSpec, foundPath string, returnErr error) {
	glog.V(3).Infof("starting find pod spec in resource %s %s/%s", podController.GetKind(), podController.GetNamespace(), podController.GetName())
	// Types are used here to help searchFields below be more readable than an
	// anonymous slice of slices of strings.
	type fields []string
	type listOfFields []fields
	searchFields := listOfFields{ // Where to search for a pod specification
		{"spec"},                     // stand-alone pod
		{"spec", "jobTemplate"},      // CronJob
		{"spec", "template", "spec"}, // Deployment, Job, and others
	}

	for _, currentFields := range searchFields {
		glog.V(5).Infof("attempting to match pod spec in fields %v of resource %s %s/%s", currentFields, podController.GetKind(), podController.GetNamespace(), podController.GetName())
		podSpecAsInterface, podSpecMatched, err := unstructured.NestedMap(podController.UnstructuredContent(), currentFields...)
		if err == nil && podSpecMatched {
			// Something exists at this field path, now convert it to a structured
			// pod type and make sure has containers.
			foundPath = strings.Join(currentFields, "/") // Save the path of the pod spec
			// THis conversion typically succeeds even if there is no actual pod
			// spec. :(
			err = runtime.DefaultUnstructuredConverter.
				FromUnstructured(podSpecAsInterface, &podSpec)
			if err == nil && len(podSpec.Containers) > 0 {
				glog.V(3).Infof("finished find pod spec in resource %s %s/%s: found in path %q", podController.GetKind(), podController.GetNamespace(), podController.GetName(), foundPath)
				return // uses named return arguments in func definition
			}
			// There was an error converting to a structured pod type.
			// THis is not a hard failure because there may be other matches in
			// searchFields.
			glog.V(5).Info("soft failure converting podSpec interface to a structured pod: found %d containers, error = %v, pod spec interface is: %v", len(podSpec.Containers), err, podSpecAsInterface)
		}
	}
	// By this point, no pod spec was matched in the Unstructured resource, or
	// able to be converted to a structured pod type.
	glog.V(3).Infof("finished find pod spec in resource %s %s/%s (unsuccessful)", podController.GetKind(), podController.GetNamespace(), podController.GetName())
	returnErr = fmt.Errorf("no pod spec found in %s %s/%s", podController.GetKind(), podController.GetNamespace(), podController.GetName())
	return // uses named return arguments in func definition
}

// FindContainerInPodSpec returns the named container from a core/v1.PodSpec
//resource.
func FindContainerInPodSpec(podSpec *core.PodSpec, containerName string) (containerResource *core.Container, containerNumber int, foundContainer bool) {
	for i, c := range podSpec.Containers {
		if c.Name == containerName {
			foundContainer = true
			containerNumber = i
			containerResource = &c
			return // uses named return arguments in func definition
		}
	}
	return // uses named return arguments in func definition
}

func FindContainerInUnstructured(u *unstructured.Unstructured, containerName string) (containerResource *core.Container, containerNumber int, foundContainer bool) {
	podSpec, _, err := FindPodSpec(u)
	if err != nil {
		// REturning false because a pod-spec could not be found in the Unstructured
		// object. We could propagate the error instead?
		return // uses named return arguments in func definition
	}
	containerResource, _, foundContainer = FindContainerInPodSpec(podSpec, containerName)
	return // uses named return arguments in func definition
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
		glog.Errorf("unable to convert resource-request multiplier to type inf.Dec, input=%f, stringified=%q, and the result is %v", multiplier, multiplierAsString, x)
		return &resource.Quantity{}, fmt.Errorf("unable to convert multiplier to type inf.Dec, input=%f, stringified=%q, and the result is %v", multiplier, multiplierAsString, x)
	}
	productAsDec := new(inf.Dec)
	productAsDec.Mul(qAsDec, multiplierAsDec)
	product := resource.NewDecimalQuantity(*productAsDec, q.Format) // Retain format from the original quantity.
	glog.V(5).Infof("multiplying resource quantity %s * %.2f = %s", q, multiplier, product)
	return product, nil
}
