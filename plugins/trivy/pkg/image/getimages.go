package image

import (
	"context"
	"fmt"
	"strings"

	"github.com/samber/lo"
	"github.com/sirupsen/logrus"

	fwControllerUtils "github.com/fairwindsops/controller-utils/pkg/controller"
	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/models"
	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
	kubeClientResources := util.CreateKubeClientResources()

	client := fwControllerUtils.Client{
		Context:    ctx,
		Dynamic:    kubeClientResources.DynamicClient,
		RESTMapper: kubeClientResources.RESTMapper,
	}

	controllers, err := client.GetAllTopControllersWithPods("")
	if err != nil {
		return nil, fmt.Errorf("could not retrieve top controllers with pods: %w", err)
	}

	// TODO: we're deduping by owner, which works in most cases, but might cause us
	// to miss certain images. E.g. mid-release, the new pods and the old pods
	// will exist under the same owner.
	keyToImage := map[string]models.Image{}
	imageOwners := map[string]map[models.Resource]struct{}{}
	for _, controller := range controllers {
		if namespaceIsBlocked(controller.TopController.GetNamespace(), namespaceBlocklist, namespaceAllowlist) {
			logrus.Debugf("Namespace %s blocked", controller.TopController.GetNamespace())
			continue
		}

		owner := models.Resource{
			Namespace: controller.TopController.GetNamespace(),
			Kind:      controller.TopController.GetKind(),
			Name:      controller.TopController.GetName(),
		}

		for _, p := range controller.Pods {
			podUnstructuredContent := p.UnstructuredContent()
			var pod corev1.Pod
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(podUnstructuredContent, &pod)
			if err != nil {
				logrus.Warnf("Unable to retrieve structured pod data: %v", err)
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

				imageID = strings.TrimPrefix(imageID, DockerIOprefix)
				imageName = strings.TrimPrefix(imageName, DockerIOprefix)

				keyToImage[imgKey] = models.Image{
					ID:      imageID,
					Name:    imageName,
					PullRef: imagePullRef,
				}
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
