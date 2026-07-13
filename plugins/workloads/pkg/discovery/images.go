package discovery

import (
	"context"
	"fmt"
	"sort"
	"strings"

	fwControllerUtils "github.com/fairwindsops/controller-utils/pkg/controller"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

const dockerIOPrefix = "index.docker.io/"

// ListRunningImages returns container images discovered across the cluster for repository inventory.
// It includes Running pods for all controllers, plus Succeeded/Failed pods for CronJob and Job
// owners so short-lived batch workloads still appear after completion. CronJob-owned Jobs are
// attributed to the CronJob. controllers should be GetAllTopControllersWithPods when available.
func ListRunningImages(ctx context.Context, kubeClient kubernetes.Interface, controllers []fwControllerUtils.Workload) (Result, error) {
	if kubeClient == nil {
		return Result{}, fmt.Errorf("kubernetes client is required")
	}

	seenPods, keyToImage, imageOwners := recordControllerPods(controllers)

	namespaces, err := listAllNamespaces(ctx, kubeClient)
	if err != nil {
		return Result{}, err
	}
	seenPods, keyToImage, imageOwners, err = recordOrphanPods(ctx, kubeClient, namespaces, seenPods, keyToImage, imageOwners)
	if err != nil {
		return Result{}, err
	}
	_, keyToImage, imageOwners, err = recordJobPods(ctx, kubeClient, namespaces, seenPods, keyToImage, imageOwners)
	if err != nil {
		return Result{}, err
	}

	return Result{
		Images: finalizeImages(keyToImage, imageOwners),
	}, nil
}

func recordControllerPods(
	controllers []fwControllerUtils.Workload,
) (
	seenPods map[string]struct{},
	keyToImage map[string]ImageResult,
	imageOwners map[string]map[string]OwnerResult,
) {
	seenPods = map[string]struct{}{}
	keyToImage = map[string]ImageResult{}
	imageOwners = map[string]map[string]OwnerResult{}

	for _, controller := range controllers {
		owner := ownerFromController(controller)

		for _, podObj := range controller.Pods {
			var pod corev1.Pod
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(podObj.UnstructuredContent(), &pod); err != nil {
				logrus.Warnf("Unable to retrieve structured pod data: %v", err)
				continue
			}
			if !podPhaseContributesImages(pod.Status.Phase, owner.Kind) {
				continue
			}
			seenPods = markPodSeen(seenPods, pod.Namespace, pod.Name)

			for _, status := range containerStatusesFromPod(pod) {
				keyToImage, imageOwners = recordContainerImage(status, owner, keyToImage, imageOwners)
			}
		}
	}
	return seenPods, keyToImage, imageOwners
}

// podPhaseContributesImages reports whether a pod phase should feed Images[].
// Long-running controllers only contribute while Running. CronJob/Job contribute
// any phase (empty ImageID is still skipped) so completed/pending batch pods can
// populate the repository when digests are present.
func podPhaseContributesImages(phase corev1.PodPhase, ownerKind string) bool {
	if phase == corev1.PodRunning {
		return true
	}
	return ownerKind == "CronJob" || ownerKind == "Job"
}

// ownerFromController identifies a top-controller owner. Label maps are omitted because
// the workloads report Controllers[] section is the canonical source for controller metadata.
func ownerFromController(controller fwControllerUtils.Workload) OwnerResult {
	return OwnerResult{
		Namespace: controller.TopController.GetNamespace(),
		Kind:      controller.TopController.GetKind(),
		Name:      controller.TopController.GetName(),
	}
}

func ownerKey(owner OwnerResult) string {
	return owner.Namespace + "/" + owner.Kind + "/" + owner.Name + "/" + owner.Container
}

func finalizeImages(keyToImage map[string]ImageResult, imageOwners map[string]map[string]OwnerResult) []ImageResult {
	images := make([]ImageResult, 0, len(keyToImage))
	for key, image := range keyToImage {
		if owners, ok := imageOwners[key]; ok {
			image.Owners = sortedOwners(owners)
		}
		images = append(images, image)
	}
	sort.Slice(images, func(i, j int) bool {
		if images[i].Name != images[j].Name {
			return images[i].Name < images[j].Name
		}
		return images[i].ID < images[j].ID
	})
	return images
}

func sortedOwners(owners map[string]OwnerResult) []OwnerResult {
	result := make([]OwnerResult, 0, len(owners))
	for _, owner := range owners {
		result = append(result, owner)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Namespace != result[j].Namespace {
			return result[i].Namespace < result[j].Namespace
		}
		if result[i].Kind != result[j].Kind {
			return result[i].Kind < result[j].Kind
		}
		if result[i].Name != result[j].Name {
			return result[i].Name < result[j].Name
		}
		return result[i].Container < result[j].Container
	})
	return result
}

func markPodSeen(seen map[string]struct{}, namespace, name string) map[string]struct{} {
	seen[podKey(namespace, name)] = struct{}{}
	return seen
}

func podKey(namespace, name string) string {
	return namespace + "/" + name
}

func recordOrphanPods(
	ctx context.Context,
	client kubernetes.Interface,
	namespaces []string,
	seenPods map[string]struct{},
	keyToImage map[string]ImageResult,
	imageOwners map[string]map[string]OwnerResult,
) (
	map[string]struct{},
	map[string]ImageResult,
	map[string]map[string]OwnerResult,
	error,
) {
	for _, namespace := range namespaces {
		pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{FieldSelector: "status.phase=Running"})
		if err != nil {
			return seenPods, keyToImage, imageOwners, fmt.Errorf("listing pods in namespace %s: %w", namespace, err)
		}
		for _, pod := range pods.Items {
			if _, ok := seenPods[podKey(pod.Namespace, pod.Name)]; ok {
				continue
			}
			if hasControllerOwner(pod.OwnerReferences) {
				continue
			}
			owner := OwnerResult{
				Namespace:      pod.Namespace,
				Kind:           "Pod",
				Name:           pod.Name,
				Labels:         pod.Labels,
				Annotations:    pod.Annotations,
				PodLabels:      pod.Labels,
				PodAnnotations: pod.Annotations,
			}
			seenPods = markPodSeen(seenPods, pod.Namespace, pod.Name)
			for _, status := range containerStatusesFromPod(pod) {
				keyToImage, imageOwners = recordContainerImage(status, owner, keyToImage, imageOwners)
			}
		}
	}
	return seenPods, keyToImage, imageOwners, nil
}

func recordJobPods(
	ctx context.Context,
	client kubernetes.Interface,
	namespaces []string,
	seenPods map[string]struct{},
	keyToImage map[string]ImageResult,
	imageOwners map[string]map[string]OwnerResult,
) (
	map[string]struct{},
	map[string]ImageResult,
	map[string]map[string]OwnerResult,
	error,
) {
	for _, namespace := range namespaces {
		jobs, err := client.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return seenPods, keyToImage, imageOwners, fmt.Errorf("listing jobs in namespace %s: %w", namespace, err)
		}
		for _, job := range jobs.Items {
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
				return seenPods, keyToImage, imageOwners, fmt.Errorf("listing job pods in namespace %s: %w", namespace, err)
			}
			owner := ownerFromJob(job)
			for _, pod := range pods.Items {
				if !podPhaseContributesImages(pod.Status.Phase, owner.Kind) {
					continue
				}
				if _, ok := seenPods[podKey(pod.Namespace, pod.Name)]; ok {
					continue
				}
				seenPods = markPodSeen(seenPods, pod.Namespace, pod.Name)
				for _, status := range containerStatusesFromPod(pod) {
					keyToImage, imageOwners = recordContainerImage(status, owner, keyToImage, imageOwners)
				}
			}
		}
	}
	return seenPods, keyToImage, imageOwners, nil
}

// ownerFromJob attributes images to the owning CronJob when present; otherwise to the Job.
// CronJob owners omit label maps (same data lives on Controllers[]). Standalone Jobs keep them.
func ownerFromJob(job batchv1.Job) OwnerResult {
	if ref := cronJobOwnerRef(job.OwnerReferences); ref != nil {
		return OwnerResult{
			Namespace: job.Namespace,
			Kind:      "CronJob",
			Name:      ref.Name,
		}
	}
	return OwnerResult{
		Namespace:      job.Namespace,
		Kind:           "Job",
		Name:           job.Name,
		Labels:         job.Labels,
		Annotations:    job.Annotations,
		PodLabels:      job.Spec.Template.Labels,
		PodAnnotations: job.Spec.Template.Annotations,
	}
}

func cronJobOwnerRef(owners []metav1.OwnerReference) *metav1.OwnerReference {
	for i := range owners {
		if owners[i].Kind == "CronJob" {
			return &owners[i]
		}
	}
	return nil
}

func hasControllerOwner(owners []metav1.OwnerReference) bool {
	for _, owner := range owners {
		if owner.Controller != nil && *owner.Controller {
			return true
		}
	}
	return false
}

func listAllNamespaces(ctx context.Context, client kubernetes.Interface) ([]string, error) {
	list, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("listing namespaces: %w", err)
	}
	namespaces := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		namespaces = append(namespaces, item.Name)
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

func recordContainerImage(
	status corev1.ContainerStatus,
	owner OwnerResult,
	keyToImage map[string]ImageResult,
	imageOwners map[string]map[string]OwnerResult,
) (map[string]ImageResult, map[string]map[string]OwnerResult) {
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

	if imageID == "" {
		logrus.Warnf("skipping container %s image %s: empty ImageID after normalization", status.Name, status.Image)
		return keyToImage, imageOwners
	}

	key := imageName + "/" + imageID
	if imageOwners[key] == nil {
		imageOwners[key] = map[string]OwnerResult{}
	}
	imageOwners[key][ownerKey(owner)] = owner
	if _, found := keyToImage[key]; found {
		return keyToImage, imageOwners
	}

	keyToImage[key] = ImageResult{
		Name:    imageName,
		ID:      imageID,
		PullRef: imagePullRef,
	}
	return keyToImage, imageOwners
}
