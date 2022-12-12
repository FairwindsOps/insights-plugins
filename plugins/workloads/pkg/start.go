package main

import (
	"context"
	"encoding/json"
	"flag"
	"os"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	ctrl "sigs.k8s.io/controller-runtime"
)

func main() {
	ctx := context.Background()
	auditOutputFile := flag.String("output-file", "", "Destination file for audit results")
	flag.Parse()

	dynamic, restMapper, kube, clusterName, err := getKubeClient()
	if err != nil {
		panic(err)
	}

	logrus.Info("connected to kube")

	resources, err := CreateResourceProviderFromAPI(ctx, dynamic, restMapper, kube, clusterName)

	if err != nil {
		logrus.Fatalf("Error fetching Kubernetes resources %v", err)
	}
	logrus.Info("got resources")

	var outputBytes []byte

	outputBytes, err = json.MarshalIndent(resources, "", "  ")

	if err != nil {
		logrus.Fatalf("Error marshalling audit: %v", err)
	}

	if *auditOutputFile != "" {
		err := os.WriteFile(*auditOutputFile, []byte(outputBytes), 0644)
		if err != nil {
			logrus.Fatalf("Error writing output to file: %v", err)
		}
	}
}

func getKubeClient() (dynamic.Interface, meta.RESTMapper, kubernetes.Interface, string, error) {
	var restMapper meta.RESTMapper
	var dynamicClient dynamic.Interface
	var kube kubernetes.Interface
	kubeConf, configError := ctrl.GetConfig()
	if configError != nil {
		logrus.Errorf("Error fetching KubeConfig: %v", configError)
		return dynamicClient, restMapper, kube, kubeConf.Host, configError
	}

	kube, err := kubernetes.NewForConfig(kubeConf)
	if err != nil {
		logrus.Errorf("Error creating Kubernetes client: %v", err)
		return dynamicClient, restMapper, kube, kubeConf.Host, err
	}

	dynamicClient, err = dynamic.NewForConfig(kubeConf)
	if err != nil {
		logrus.Errorf("Error creating Dynamic client: %v", err)
		return dynamicClient, restMapper, kube, kubeConf.Host, err
	}

	resources, err := restmapper.GetAPIGroupResources(kube.Discovery())
	if err != nil {
		logrus.Errorf("Error getting API Group resources: %v", err)
		return dynamicClient, restMapper, kube, kubeConf.Host, err
	}
	restMapper = restmapper.NewDiscoveryRESTMapper(resources)
	return dynamicClient, restMapper, kube, kubeConf.Host, nil
}
