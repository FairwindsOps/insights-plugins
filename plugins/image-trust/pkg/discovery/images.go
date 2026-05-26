package discovery

import (
	"context"
	"fmt"
	"strings"

	fwControllerUtils "github.com/fairwindsops/controller-utils/pkg/controller"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	ctrl "sigs.k8s.io/controller-runtime"
)

const dockerIOPrefix = "index.docker.io/"

type kubeClientResources struct {
	DynamicClient dynamic.Interface
	RESTMapper    meta.RESTMapper
	KubeClient    kubernetes.Interface
}

// ListImages returns the images currently used by workloads in the cluster.
func ListImages(ctx context.Context, namespaceBlocklist, namespaceAllowlist []string) ([]models.DiscoveredImage, error) {
	kubeResources, err := createKubeClientResources()
	if err != nil {
		return nil, err
	}

	client := fwControllerUtils.Client{
		Context:    ctx,
		Dynamic:    kubeResources.DynamicClient,
		RESTMapper: kubeResources.RESTMapper,
	}

	controllers, err := client.GetAllTopControllersWithPods("")
	if err != nil {
		return nil, fmt.Errorf("could not retrieve top controllers with pods: %w", err)
	}

	keyToImage := map[string]models.DiscoveredImage{}
	imageOwners := map[string]map[models.Resource]struct{}{}
	for _, controller := range controllers {
		namespace := strings.ToLower(controller.TopController.GetNamespace())
		if namespaceIsBlocked(namespace, namespaceBlocklist, namespaceAllowlist) {
			logrus.Debugf("Namespace %s blocked", namespace)
			continue
		}

		owner := models.Resource{
			Namespace: controller.TopController.GetNamespace(),
			Kind:      controller.TopController.GetKind(),
			Name:      controller.TopController.GetName(),
		}

		for _, podObj := range controller.Pods {
			var pod corev1.Pod
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(podObj.UnstructuredContent(), &pod); err != nil {
				logrus.Warnf("Unable to retrieve structured pod data: %v", err)
				continue
			}

			for _, status := range pod.Status.ContainerStatuses {
				imageName := status.Image
				if strings.HasPrefix(status.Image, "sha256") {
					imageName = strings.TrimPrefix(status.ImageID, "docker-pullable://")
				}

				imageID := strings.TrimPrefix(status.ImageID, "docker-pullable://")
				imagePullRef := imageID
				if imagePullRef == "" || strings.HasPrefix(imagePullRef, "sha256:") {
					imagePullRef = imageName
				}

				owner.Container = status.Name
				imageName = strings.TrimPrefix(imageName, dockerIOPrefix)
				imageID = strings.TrimPrefix(imageID, dockerIOPrefix)

				key := imageName + "/" + imageID
				if imageOwners[key] == nil {
					imageOwners[key] = map[models.Resource]struct{}{}
				}
				imageOwners[key][owner] = struct{}{}
				if _, found := keyToImage[key]; found {
					continue
				}

				keyToImage[key] = models.DiscoveredImage{
					Name:    imageName,
					ID:      imageID,
					PullRef: imagePullRef,
				}
			}
		}
	}

	for key, image := range keyToImage {
		if owners, ok := imageOwners[key]; ok {
			image.Owners = lo.Keys(owners)
			keyToImage[key] = image
		}
	}

	return lo.Values(keyToImage), nil
}

func namespaceIsBlocked(namespace string, namespaceBlocklist, namespaceAllowlist []string) bool {
	for _, blocked := range namespaceBlocklist {
		if namespace == blocked {
			return true
		}
	}
	if len(namespaceAllowlist) == 0 {
		return false
	}
	for _, allowed := range namespaceAllowlist {
		if namespace == allowed {
			return false
		}
	}
	return true
}

func createKubeClientResources() (*kubeClientResources, error) {
	kubeConfig, err := ctrl.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("fetching kube config: %w", err)
	}

	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("creating Kubernetes client: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	groupResources, err := restmapper.GetAPIGroupResources(kubeClient.Discovery())
	if err != nil {
		return nil, fmt.Errorf("getting API group resources: %w", err)
	}

	return &kubeClientResources{
		DynamicClient: dynamicClient,
		RESTMapper:    restmapper.NewDiscoveryRESTMapper(groupResources),
		KubeClient:    kubeClient,
	}, nil
}
