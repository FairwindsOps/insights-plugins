package util

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/golang/glog"

	"strings"

	"encoding/json"

	fwControllerUtils "github.com/fairwindsops/controller-utils/pkg/controller"
	"gopkg.in/inf.v0"
	core "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

// CreateKubeClientResources returns a KubeClientResources type, trying first
// in-cluster, then local, KubeConfig.
func CreateKubeClientResources() KubeClientResources {
	// Try to load in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		glog.V(3).Infof("Could not load in-cluster config, falling back to $KUBECONFIG or ~/.kube/config: %v", err)
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
	httpClient, err := rest.HTTPClientFor(config)
	if err != nil {
		glog.Fatal("error creating httpClient using kubeconfig: %s", err.Error())
	}
	RESTMapper, err := apiutil.NewDynamicRESTMapper(config, httpClient)
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

// findPodSpecInUnstructured searches an unstructured.Unstructured resource for a pod
// specification, returning it as a core.podspec type, along with the path
// where the pod spec was found (such as spec.template.spec).
func FindPodSpecInUnstructured(podController *unstructured.Unstructured) (podSpec *core.PodSpec, foundPath string, returnErr error) {
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
				return
			}
			// There was an error converting to a structured pod type.
			// THis is not a hard failure because there may be other matches in
			// searchFields.
			glog.V(5).Infof("soft failure converting podSpec interface to a structured pod: found %d containers, error = %v, pod spec interface is: %v", len(podSpec.Containers), err, podSpecAsInterface)
		}
	}
	// By this point, no pod spec was matched in the Unstructured resource, or
	// able to be converted to a structured pod type.
	glog.V(3).Infof("finished find pod spec in resource %s %s/%s (unsuccessful)", podController.GetKind(), podController.GetNamespace(), podController.GetName())
	returnErr = fmt.Errorf("no pod spec found in %s %s/%s", podController.GetKind(), podController.GetNamespace(), podController.GetName())
	return
}

// FindContainerInPodSpec finds a named container in a core/v1.PodSpec
// resource.
func FindContainerInPodSpec(podSpec *core.PodSpec, containerName string) (containerResource *core.Container, containerNumber int, foundContainer bool) {
	for i, c := range podSpec.Containers {
		if c.Name == containerName {
			foundContainer = true
			containerNumber = i
			containerResource = &c
			return
		}
	}
	return
}

// FindContainerInUnstructured is a wrapper around FindPodSpecInUnstructured
// and FindContainerInPodSpec.
func FindContainerInUnstructured(u *unstructured.Unstructured, containerName string) (containerResource *core.Container, podSpecPath string, containerNumber int, foundContainer bool, returnedErr error) {
	podSpec, podSpecPath, err := FindPodSpecInUnstructured(u)
	if err != nil {
		returnedErr = fmt.Errorf("unable to find podspec to find container %q: %w", containerName, err)
		return
	}
	containerResource, containerNumber, foundContainer = FindContainerInPodSpec(podSpec, containerName)
	return
}

// patchContainerMemoryLimits patches the named container with a new
// resources.limits.memory. The resource to be patched is of type
// unstructured.Unstructured, to support multiple Kinds that have a pod spec, using
// FindPodSpecInUnstructured() to locate the pod spec and containers.
func PatchContainerMemoryLimits(kubeClientResources KubeClientResources, resource *unstructured.Unstructured, containerName string, newContainerMemoryLimits *resource.Quantity) (*unstructured.Unstructured, error) {
	glog.V(2).Infof("starting patch %s %s/%s:%s memory limits to %s", resource.GetNamespace(), resource.GetKind(), resource.GetName(), containerName, newContainerMemoryLimits)
	_, podSpecPath, containerNumber, foundContainer, err := FindContainerInUnstructured(resource, containerName)
	if err != nil {
		return nil, err
	}
	if !foundContainer {
		return nil, fmt.Errorf("no container %q found in resource %v", containerName, resource)
	}
	patch := []interface{}{
		map[string]interface{}{
			"op":    "replace",
			"path":  "/" + podSpecPath + "/containers/" + strconv.Itoa(containerNumber) + "/resources/limits/memory",
			"value": newContainerMemoryLimits.String(),
		},
	}
	patchJSON, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal patch %v: %v", patch, err)
	}
	// A GroupVersionKind is required to create the RESTMapping, which maps;
	// converts the API version and kind to the correct capitolization and plural
	// syntax required by the Kube API.
	GVK := resource.GroupVersionKind()
	GVKMapping, err := kubeClientResources.RESTMapper.RESTMapping(GVK.GroupKind(), GVK.Version)
	if err != nil {
		return nil, fmt.Errorf("error creating RESTMapper mapping from group-version-kind %v: %v", GVK, err)
	}
	patchClient := kubeClientResources.DynamicClient.Resource(GVKMapping.Resource).Namespace(resource.GetNamespace())
	glog.V(2).Infof("going to patch %s/%s: %#v", resource.GetNamespace(), resource.GetName(), string(patchJSON))
	patchedResource, err := patchClient.Patch(context.TODO(), resource.GetName(), types.JSONPatchType, patchJSON, metav1.PatchOptions{})
	if err != nil {
		return nil, fmt.Errorf("error patching %s %s/%s: %v", resource.GetKind(), resource.GetNamespace(), resource.GetName(), err)
	}
	glog.V(4).Infof("resource after patch is: %v", patchedResource)
	glog.V(2).Infof("finished patch %s %s/%s:%s memory limits to %s", resource.GetNamespace(), resource.GetKind(), resource.GetName(), containerName, newContainerMemoryLimits)
	return patchedResource, nil
}

// GetUnstructuredResourceFromObjectRef fffetches a resource from in-cluster, based on
// the GroupVersionKind from the provided core.ObjectReference. An
// ObjectReference is typically included in a Kube Event.
func GetUnstructuredResourceFromObjectRef(kubeClientResources KubeClientResources, objectRef core.ObjectReference) (resource *unstructured.Unstructured, found bool, err error) {
	// A GroupVersionKind is required to create the RESTMapping, which maps;
	// converts the API version and kind to the correct capitolization and plural
	// syntax required by the Kube API.
	GVK := objectRef.GroupVersionKind()
	GVKMapping, err := kubeClientResources.RESTMapper.RESTMapping(GVK.GroupKind(), GVK.Version)
	if err != nil {
		return nil, false, fmt.Errorf("error creating RESTMapper mapping from group-version-kind %v: %v", GVK, err)
	}
	getterClient := kubeClientResources.DynamicClient.Resource(GVKMapping.Resource).Namespace(objectRef.Namespace)
	glog.V(3).Infof("starting fetch resource %s %s/%s in-cluster to get the full resource from an objectRef", objectRef.Kind, objectRef.Namespace, objectRef.Name)
	resource, err = getterClient.Get(context.TODO(), objectRef.Name, metav1.GetOptions{})
	if k8sErrors.IsNotFound(err) {
		glog.V(3).Infof("resource %s %s/%s not found in-cluster while getting the full resource from an objectRef", objectRef.Kind, objectRef.Namespace, objectRef.Name)
		return nil, false, nil
	}
	if err != nil {
		glog.V(3).Infof("error fetching %s %s/%s in-cluster while getting the full resource from an objectRef: %v", objectRef.Kind, objectRef.Namespace, objectRef.Name, err)
		return nil, false, err
	}
	glog.V(3).Infof("finished fetch resource %s %s/%s in-cluster to get the full resource from an objectRef", objectRef.Kind, objectRef.Namespace, objectRef.Name)
	return resource, true, nil
}

// GetControllerFromPod accepts a typed pod object, and returns the pod-controller
// which owns the pod.
// E.G. an owning pod-controller might be a Kubernetes Deployment, DaemonSet,
// or CronJob.
func GetControllerFromPod(kubeClientResources KubeClientResources, pod *core.Pod) (*unstructured.Unstructured, error) {
	// Convert a pod type to an unstructured one.
	podJSON, err := json.Marshal(pod)
	if err != nil {
		return nil, err
	}
	objectAsMap := make(map[string]interface{})
	err = json.Unmarshal(podJSON, &objectAsMap)
	if err != nil {
		return nil, err
	}
	unstructuredPod := unstructured.Unstructured{
		Object: objectAsMap,
	}

	client := fwControllerUtils.Client{
		Context: context.TODO(),
		Dynamic: kubeClientResources.DynamicClient,
		RESTMapper: kubeClientResources.RESTMapper,
	}
	topController, err := client.GetTopController(unstructuredPod, nil)
	if err != nil {
		return nil, err
	}
	glog.V(2).Infof("found controller kind %q named %q for pod %s/%s", topController.GetKind(), topController.GetName(), pod.Namespace, pod.Name)
	return &topController, nil
}

// MultiplyResourceQuantity multiplies a
// k8s.io/apimachinery/pkg/api/resource.Quantity with a float64, returning a
// new resource.Quantity.
// If an error is returned, the resource.Quantity will be its zero value.
// To avoid losing precision and occasional failures converting to int64, the resource.Quantity and multiplier are
// converted to inf.Dec types to perform multiplication.
// For reference, see the quantity.AsInt64 documentation: https://pkg.go.dev/k8s.io/apimachinery/pkg/api/resource#Quantity.AsInt64
// Note that certain multipliers cause the units of the result to become
// smaller, I.E. from Gi to Mi or Ki.
func MultiplyResourceQuantity(q *resource.Quantity, multiplier float64) (*resource.Quantity, error) {
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
