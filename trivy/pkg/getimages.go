package main

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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
		}
		if len(pod.ObjectMeta.OwnerReferences) == 1 {
			ownerInfo := pod.ObjectMeta.OwnerReferences[0]
			owner.Kind = ownerInfo.Kind
			owner.Name = ownerInfo.Name
		} else {
			owner.Kind = "Pod"
			owner.Name = pod.ObjectMeta.Name
		}
		key := owner.Namespace + "/" + owner.Kind + "/" + owner.Name
		if _, ok := found[key]; ok {
			continue
		}
		found[key] = true
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
			images = append(images, im)
		}
	}
	return images, nil
}
