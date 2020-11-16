package main

import (
	"context"
	"os"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/fairwindsops/insights-plugins/prometheus/pkg/data"
)

func main() {
	address := os.Getenv("PROMETHEUS_ADDRESS")
	client, err := data.GetClient(address)
	if err != nil {
		panic(err)
	}
	hostName := os.Getenv("FAIRWINDS_INSIGHTS_HOST")
	org := os.Getenv("FAIRWINDS_ORG")
	cluster := os.Getenv("FAIRWINDS_CLUSTER")
	token := os.Getenv("FAIRWINDS_TOKEN")
	if hostName == "" || org == "" || cluster == "" || token == "" {
		panic("Proper environment variables not set.")
	}

	dynamic, restMapper, err := getKubeClient()
	if err != nil {
		panic(err)
	}
	res, err := data.GetMetrics(context.Background(), dynamic, restMapper, client)
	if err != nil {
		panic(err)
	}
	stats := data.CalculateStatistics(res)
	err = data.SubmitMetrics(stats, hostName, org, cluster, token)
	if err != nil {
		panic(err)
	}

}

func getKubeClient() (dynamic.Interface, meta.RESTMapper, error) {
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
