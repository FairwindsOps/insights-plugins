package discovery

import (
	"context"
	"fmt"
	"strings"

	fwControllerUtils "github.com/fairwindsops/controller-utils/pkg/controller"
	"github.com/fairwindsops/insights-plugins/plugins/image-trust/pkg/models"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// ListImages returns images used by workloads and additional running pods in scope.
func ListImages(ctx context.Context, kubeClient kubernetes.Interface, namespaceBlocklist, namespaceAllowlist []string) (Result, error) {
	if kubeClient == nil {
		return Result{}, fmt.Errorf("kubernetes client is required")
	}

	kubeResources, err := kubeClientResourcesFrom(kubeClient)
	if err != nil {
		return Result{}, err
	}

	client := fwControllerUtils.Client{
		Context:    ctx,
		Dynamic:    kubeResources.DynamicClient,
		RESTMapper: kubeResources.RESTMapper,
	}

	controllers, err := client.GetAllTopControllersWithPods("")
	if err != nil {
		return Result{}, fmt.Errorf("could not retrieve top controllers with pods: %w", err)
	}

	keyToImage := map[string]models.DiscoveredImage{}
	imageOwners := map[string]map[models.Resource]struct{}{}
	seenPods := map[string]struct{}{}

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
			markPodSeen(seenPods, pod.Namespace, pod.Name)

			for _, status := range containerStatusesFromPod(pod) {
				recordContainerImage(status, owner, keyToImage, imageOwners)
			}
		}
	}

	namespaces, err := listScopedNamespaces(ctx, kubeResources.KubeClient, namespaceAllowlist, namespaceBlocklist)
	if err != nil {
		return Result{}, err
	}
	if err := recordOrphanPods(ctx, kubeResources.KubeClient, namespaces, seenPods, keyToImage, imageOwners); err != nil {
		return Result{}, err
	}
	if err := recordJobPods(ctx, kubeResources.KubeClient, namespaces, seenPods, keyToImage, imageOwners); err != nil {
		return Result{}, err
	}

	for key, image := range keyToImage {
		if owners, ok := imageOwners[key]; ok {
			image.Owners = lo.Keys(owners)
			keyToImage[key] = image
		}
	}

	return Result{
		Images: lo.Values(keyToImage),
	}, nil
}

func markPodSeen(seen map[string]struct{}, namespace, name string) {
	seen[podKey(namespace, name)] = struct{}{}
}

func podKey(namespace, name string) string {
	return namespace + "/" + name
}

func recordOrphanPods(
	ctx context.Context,
	client kubernetes.Interface,
	namespaces []string,
	seenPods map[string]struct{},
	keyToImage map[string]models.DiscoveredImage,
	imageOwners map[string]map[models.Resource]struct{},
) error {
	for _, namespace := range namespaces {
		pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{FieldSelector: "status.phase=Running"})
		if err != nil {
			return fmt.Errorf("listing pods in namespace %s: %w", namespace, err)
		}
		for _, pod := range pods.Items {
			if _, ok := seenPods[podKey(pod.Namespace, pod.Name)]; ok {
				continue
			}
			if hasControllerOwner(pod.OwnerReferences) {
				continue
			}
			owner := models.Resource{
				Namespace: pod.Namespace,
				Kind:      "Pod",
				Name:      pod.Name,
			}
			markPodSeen(seenPods, pod.Namespace, pod.Name)
			for _, status := range containerStatusesFromPod(pod) {
				recordContainerImage(status, owner, keyToImage, imageOwners)
			}
		}
	}
	return nil
}

func recordJobPods(
	ctx context.Context,
	client kubernetes.Interface,
	namespaces []string,
	seenPods map[string]struct{},
	keyToImage map[string]models.DiscoveredImage,
	imageOwners map[string]map[models.Resource]struct{},
) error {
	for _, namespace := range namespaces {
		jobs, err := client.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("listing jobs in namespace %s: %w", namespace, err)
		}
		for _, job := range jobs.Items {
			if !jobHasActivePods(job) {
				continue
			}
			if job.Spec.Selector == nil {
				continue
			}
			selector, err := metav1.LabelSelectorAsSelector(job.Spec.Selector)
			if err != nil {
				continue
			}
			pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: selector.String(),
			})
			if err != nil {
				return fmt.Errorf("listing job pods in namespace %s: %w", namespace, err)
			}
			owner := models.Resource{
				Namespace: job.Namespace,
				Kind:      "Job",
				Name:      job.Name,
			}
			for _, pod := range pods.Items {
				if pod.Status.Phase != corev1.PodRunning {
					continue
				}
				markPodSeen(seenPods, pod.Namespace, pod.Name)
				for _, status := range containerStatusesFromPod(pod) {
					recordContainerImage(status, owner, keyToImage, imageOwners)
				}
			}
		}
	}
	return nil
}

func jobHasActivePods(job batchv1.Job) bool {
	return job.Status.Active > 0
}

func hasControllerOwner(owners []metav1.OwnerReference) bool {
	for _, owner := range owners {
		if owner.Controller != nil && *owner.Controller {
			return true
		}
	}
	return false
}

func listScopedNamespaces(
	ctx context.Context,
	client kubernetes.Interface,
	namespaceAllowlist, namespaceBlocklist []string,
) ([]string, error) {
	if len(namespaceAllowlist) > 0 {
		namespaces := make([]string, 0, len(namespaceAllowlist))
		for _, ns := range namespaceAllowlist {
			if !namespaceIsBlocked(strings.ToLower(ns), namespaceBlocklist, namespaceAllowlist) {
				namespaces = append(namespaces, ns)
			}
		}
		return namespaces, nil
	}
	list, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing namespaces: %w", err)
	}
	namespaces := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		if !namespaceIsBlocked(strings.ToLower(item.Name), namespaceBlocklist, namespaceAllowlist) {
			namespaces = append(namespaces, item.Name)
		}
	}
	return namespaces, nil
}

func containerStatusesFromPod(pod corev1.Pod) []corev1.ContainerStatus {
	statuses := make([]corev1.ContainerStatus, 0,
		len(pod.Status.ContainerStatuses)+len(pod.Status.InitContainerStatuses)+len(pod.Status.EphemeralContainerStatuses))
	statuses = append(statuses, pod.Status.ContainerStatuses...)
	statuses = append(statuses, pod.Status.InitContainerStatuses...)
	statuses = append(statuses, pod.Status.EphemeralContainerStatuses...)
	return statuses
}

func recordContainerImage(status corev1.ContainerStatus, owner models.Resource, keyToImage map[string]models.DiscoveredImage, imageOwners map[string]map[models.Resource]struct{}) {
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
		return
	}

	keyToImage[key] = models.DiscoveredImage{
		Name:    imageName,
		ID:      imageID,
		PullRef: imagePullRef,
	}
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

func kubeClientResourcesFrom(kubeClient kubernetes.Interface) (*kubeClientResources, error) {
	kubeConfig, err := ctrl.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("fetching kube config: %w", err)
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
