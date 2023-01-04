package kube

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	ctrl "sigs.k8s.io/controller-runtime"
	"github.com/sirupsen/logrus"
)

func GetKubeClient() (dynamic.Interface, meta.RESTMapper, error) {
	var restMapper meta.RESTMapper
	var dynamicClient dynamic.Interface
	kubeConf, configError := ctrl.GetConfig()
	if configError != nil {
		logrus.Errorf("Error fetching KubeConfig: %v", configError)
		return dynamicClient, restMapper, configError
	}

	api, err := kubernetes.NewForConfig(kubeConf)
	if err != nil {
		logrus.Errorf("Error creating Kubernetes client: %v", err)
		return dynamicClient, restMapper, err
	}

	dynamicClient, err = dynamic.NewForConfig(kubeConf)
	if err != nil {
		logrus.Errorf("Error creating Dynamic client: %v", err)
		return dynamicClient, restMapper, err
	}

	resources, err := restmapper.GetAPIGroupResources(api.Discovery())
	if err != nil {
		logrus.Errorf("Error getting API Group resources: %v", err)
		return dynamicClient, restMapper, err
	}
	restMapper = restmapper.NewDiscoveryRESTMapper(resources)
	return dynamicClient, restMapper, nil
}

