package image

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/models"
)

var namespaceBlocklist []string
var namespaceAllowlist []string

func init() {
	if os.Getenv("NAMESPACE_BLACKLIST") != "" {
		namespaceBlocklist = strings.Split(os.Getenv("NAMESPACE_BLACKLIST"), ",")
	}
	if os.Getenv("NAMESPACE_BLOCKLIST") != "" {
		namespaceBlocklist = strings.Split(os.Getenv("NAMESPACE_BLOCKLIST"), ",")
	}
	if os.Getenv("NAMESPACE_ALLOWLIST") != "" {
		namespaceAllowlist = strings.Split(os.Getenv("NAMESPACE_ALLOWLIST"), ",")
	}
	logrus.Infof("%d namespaces allowed, %d namespaces blocked", len(namespaceAllowlist), len(namespaceBlocklist))
}

func namespaceIsBlocked(ns string) bool {
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
func GetImages(ctx context.Context) ([]models.Image, error) {
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
	found := map[string]bool{}
	images := []models.Image{}
	for _, pod := range pods.Items {
		if namespaceIsBlocked(pod.ObjectMeta.Namespace) {
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
			im := models.Image{
				Name:  imageName,
				ID:    strings.TrimPrefix(containerStatus.ImageID, "docker-pullable://"),
				Owner: models.Resource(owner),
			}
			im.PullRef = im.ID
			if im.PullRef == "" || strings.HasPrefix(im.PullRef, "sha256:") {
				im.PullRef = im.Name
			}
			im.Owner.Container = containerStatus.Name
			key := im.Owner.Namespace + "/" + im.Owner.Kind + "/" + im.Owner.Name + "/" + im.Owner.Container + "/" + im.Name + "/" + im.ID
			if _, ok := found[key]; ok {
				continue
			}
			found[key] = true

			images = append(images, im)
		}
	}
	return images, nil
}
