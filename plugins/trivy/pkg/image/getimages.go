package image

import (
	"context"
	"fmt"
	"strings"

	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/models"
)

func namespaceIsBlocked(ns string, namespaceBlocklist, namespaceAllowlist []string) bool {
	for _, namespace := range namespaceBlocklist {
		if ns == strings.TrimSpace(strings.ToLower(namespace)) {
			return true
		}
	}
	if len(namespaceAllowlist) == 0 {
		return false
	}
	for _, namespace := range namespaceAllowlist {
		if ns == strings.TrimSpace(strings.ToLower(namespace)) {
			return false
		}
	}
	return true
}

// GetImages returns the images in the current cluster.
func GetImages(ctx context.Context, namespaceBlocklist, namespaceAllowlist []string) ([]models.Image, error) {
	kubeConf, configError := ctrl.GetConfig()
	if configError != nil {
		return nil, fmt.Errorf("Error fetching KubeConfig: %v", configError)
	}

	api, err := kubernetes.NewForConfig(kubeConf)
	if err != nil {
		return nil, fmt.Errorf("Error creating Kubernetes client: %v", err)
	}

	dynamicClient, err := dynamic.NewForConfig(kubeConf)
	if err != nil {
		return nil, fmt.Errorf("Error creating Dynamic client: %v", err)
	}

	resources, err := restmapper.GetAPIGroupResources(api.Discovery())
	if err != nil {
		return nil, fmt.Errorf("Error getting API Group resources: %v", err)
	}
	restMapper := restmapper.NewDiscoveryRESTMapper(resources)

	listOpts := metav1.ListOptions{}
	pods, err := api.CoreV1().Pods("").List(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("Error fetching Kubernetes pods: %v", err)
	}

	// TODO: we're deduping by owner, which works in most cases, but might cause us
	// to miss certain images. E.g. mid-release, the new pods and the old pods
	// will exist under the same owner.
	keyToImage := map[string]models.Image{}
	imageOwners := map[string]map[models.Resource]struct{}{}
	for _, pod := range pods.Items {
		if namespaceIsBlocked(pod.ObjectMeta.Namespace, namespaceBlocklist, namespaceAllowlist) {
			logrus.Debugf("Namespace %s blocked", pod.ObjectMeta.Namespace)
			continue
		}
		owner := models.Resource{
			Namespace: pod.ObjectMeta.Namespace,
			Kind:      "Pod",
			Name:      pod.ObjectMeta.Name,
		}
		owners := pod.ObjectMeta.OwnerReferences

		for len(owners) > 0 {
			if len(owners) > 1 {
				logrus.Warnf("More than 1 owner found for Namespace: %s Kind: %s Object: %s", owner.Namespace, owner.Kind, owner.Name)
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
			getParents, err := dynamicClient.Resource(mapping.Resource).Namespace(pod.ObjectMeta.Namespace).Get(ctx, firstOwner.Name, metav1.GetOptions{})
			if err != nil {
				logrus.Warnf("Error retrieving parent object %s of API %s and Kind %s because of error: %v ", firstOwner.Name, firstOwner.APIVersion, firstOwner.Kind, err)
				break
			}
			owners = getParents.GetOwnerReferences()
		}

		for _, containerStatus := range pod.Status.ContainerStatuses {
			var imageName string
			if strings.HasPrefix(containerStatus.Image, "sha256") {
				imageName = strings.TrimPrefix(containerStatus.ImageID, "docker-pullable://")
				logrus.Debugf("using an image name %q from the containerStatuses.*.ImageID field, because containerStatuses.*.Image begins with sha256 - %q", imageName, containerStatus.Image)
			} else {
				imageName = containerStatus.Image
			}
			imageID := strings.TrimPrefix(containerStatus.ImageID, "docker-pullable://")
			imagePullRef := imageID
			if imagePullRef == "" || strings.HasPrefix(imagePullRef, "sha256:") {
				imagePullRef = imageName
			}
			owner.Container = containerStatus.Name
			imgKey := imageName + "/" + imageID
			if imageOwners[imgKey] == nil {
				imageOwners[imgKey] = map[models.Resource]struct{}{}
			}
			imageOwners[imgKey][owner] = struct{}{}
			if _, found := keyToImage[imgKey]; found {
				continue
			}
			keyToImage[imgKey] = models.Image{
				ID:      imageID,
				Name:    imageName,
				PullRef: imagePullRef,
			}
		}
	}

	// add owners to images
	for key, image := range keyToImage {
		if owners, ok := imageOwners[key]; ok {
			image.Owners = lo.Keys(owners)
			keyToImage[key] = image
		}
	}
	return lo.Values(keyToImage), nil
}
