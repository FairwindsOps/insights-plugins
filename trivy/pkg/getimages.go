package main

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Image struct {
	Name    string
	ID      string
	PullRef string
	Owner   Resource
}
type Resource struct {
	Kind      string
	Namespace string
	Name      string
}

func GetImages() ([]Image, error) {
	kubeConf, configError := ctrl.GetConfig()
	if configError != nil {
		logrus.Errorf("Error fetching KubeConfig: %v", configError)
		return nil, configError
	}

	api, err := kubernetes.NewForConfig(kubeConf)
	if err != nil {
		logrus.Errorf("Error creating Kubernetes client: %v", err)
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(kubeConf)
	if err != nil {
		logrus.Errorf("Error creating Dynamic client: %v", err)
		return nil, err
	}

	resources, err := restmapper.GetAPIGroupResources(api.Discovery())
	if err != nil {
		logrus.Errorf("Error getting API Group resources: %v", err)
		return nil, err
	}
	restMapper := restmapper.NewDiscoveryRESTMapper(resources)

	listOpts := metav1.ListOptions{}
	pods, err := api.CoreV1().Pods("").List(listOpts)

	if err != nil {
		logrus.Errorf("Error fetching Kubernetes pods: %v", err)
		return nil, err
	}

	// TODO: we're deduping by owner, which works in most cases, but might cause us
	// to miss certain images. E.g. mid-release, the new pods and the old pods
	// will exist under the same owner.
	found := map[string]bool{}
	images := []Image{}
	namespaceBlacklist := strings.Split(os.Getenv("NAMESPACE_BLACKLIST"), ",")
	for _, pod := range pods.Items {
		foundNamespace := false
		for _, namespace := range namespaceBlacklist {
			if pod.ObjectMeta.Namespace == strings.ToLower(namespace) {
				foundNamespace = true
				break
			}
		}
		if foundNamespace {
			continue
		}
		owner := Resource{
			Namespace: pod.ObjectMeta.Namespace,
			Kind:      "Pod",
			Name:      pod.ObjectMeta.Name,
		}
		owners := pod.ObjectMeta.OwnerReferences

		for len(owners) > 0 {
			if len(owners) > 1 {
				logrus.Warnf("More than 1 owner found for Namespace: %s Kind: %s Object: %s", owner.Namespace.owner.Kind, owner.Name)
			}
			firstOwner := owners[0]
			owner.Kind = firstOwner.Kind
			owner.Name = firstOwner.Name
			if owner.Kind == "Node" {
				break
			}
			fqKind := schema.FromAPIVersionAndKind(firstOwner.APIVersion, firstOwner.Kind)
			mapping, err := restMapper.RESTMapping(fqKind.GroupKind(), fqKind.Version)
			if err != nil {
				logrus.Warnf("Error retrieving mapping %s of API %s and Kind %s because of error: %v ", firstOwner.Name, firstOwner.APIVersion, firstOwner.Kind, err)
				break
			}
			getParents, err := dynamicClient.Resource(mapping.Resource).Namespace(pod.ObjectMeta.Namespace).Get(firstOwner.Name, metav1.GetOptions{})
			if err != nil {
				logrus.Warnf("Error retrieving parent object %s of API %s and Kind %s because of error: %v ", firstOwner.Name, firstOwner.APIVersion, firstOwner.Kind, err)
				break
			}
			owners = getParents.GetOwnerReferences()

		}

		for _, containerStatus := range pod.Status.ContainerStatuses {
			im := Image{
				Name:  containerStatus.Image,
				ID:    strings.TrimPrefix(containerStatus.ImageID, "docker-pullable://"),
				Owner: Resource(owner),
			}
			im.PullRef = im.ID
			if im.PullRef == "" || strings.HasPrefix(im.PullRef, "sha256:") {
				im.PullRef = im.Name
			}
			im.Owner.Name += "/" + containerStatus.Name
			key := im.Owner.Namespace + "/" + im.Owner.Kind + "/" + im.Owner.Name + "/" + im.Name + "/" + im.ID
			if _, ok := found[key]; ok {
				continue
			}
			found[key] = true

			images = append(images, im)
		}
	}
	return images, nil
}
