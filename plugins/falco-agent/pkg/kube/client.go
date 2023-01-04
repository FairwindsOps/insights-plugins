package kube

import (
	"context"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"github.com/fairwindsops/controller-utils/pkg/controller"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"k8s.io/client-go/restmapper"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Client struct {
	context     context.Context
	restMapper  meta.RESTMapper
	dynamic     dynamic.Interface
	Controllers controller.Client
}

// GetPodByPodName returns pod from the namespace and name provided.
func (c Client) GetPodByPodName(namespace, podname string) (*unstructured.Unstructured, error) {
	fqKind := schema.FromAPIVersionAndKind("v1", "Pod")
	mapping, err := c.restMapper.RESTMapping(fqKind.GroupKind(), fqKind.Version)
	if err != nil {
		return nil, err
	}
	pod, err := c.dynamic.Resource(mapping.Resource).Namespace(namespace).Get(c.context, podname, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return pod, nil
}

func GetPodFromUnstructured(u *unstructured.Unstructured) (*corev1.Pod, error) {
	var pd corev1.Pod
	err := runtime.DefaultUnstructuredConverter.
		FromUnstructured(u.UnstructuredContent(), &pd)
	if err != nil {
		return nil, err
	}
	return &pd, nil
}

func GetKubeClient() (*Client, error) {
	var restMapper meta.RESTMapper
	var dynamicClient dynamic.Interface
	kubeConf, configError := config.GetConfig()
	if configError != nil {
		logrus.Errorf("Error fetching KubeConfig: %v", configError)
		return nil, configError
	}

	api, err := kubernetes.NewForConfig(kubeConf)
	if err != nil {
		logrus.Errorf("Error creating Kubernetes client: %v", err)
		return nil, err
	}

	dynamicClient, err = dynamic.NewForConfig(kubeConf)
	if err != nil {
		logrus.Errorf("Error creating Dynamic client: %v", err)
		return nil, err
	}

	restMapper = restmapper.NewDynamicRESTMapper(kubeConf)
	ctx := context.TODO()
	client := Client{
		context:    ctx,
		dynamic:    dynamicClient,
		restMapper: restMapper,
	}
	client.Controllers = controller.Client{
		Context:    ctx,
		Dynamic:    dynamicClient,
		RESTMapper: restMapper,
	}
	return &client, nil
}
