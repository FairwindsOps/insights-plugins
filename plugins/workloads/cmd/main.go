package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	workloads "github.com/fairwindsops/insights-plugins/plugins/workloads/pkg"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	ctrl "sigs.k8s.io/controller-runtime"
)

const tempFile = "/output/workloads-temp.json"

func main() {
	ctx := context.Background()
	auditOutputFile := flag.String("output-file", "", "Destination file for audit results")
	flag.Parse()

	dynamic, restMapper, kube, clusterName, err := getKubeClient()
	if err != nil {
		logrus.Fatalf("error fetching Kubernetes client: %v", err)
	}
	logrus.Info("connected to kube")

	resources, err := workloads.CreateResourceProviderFromAPI(ctx, dynamic, restMapper, kube, clusterName)
	if err != nil {
		logrus.Fatalf("error fetching Kubernetes resources: %v", err)
	}
	logrus.Info("got resources")

	var outputBytes []byte
	outputBytes, err = json.MarshalIndent(resources, "", "  ")
	if err != nil {
		logrus.Fatalf("error marshalling audit: %v", err)
	}

	if *auditOutputFile != "" {
		err := os.WriteFile(tempFile, outputBytes, 0644)
		if err != nil {
			logrus.Fatalf("error writing output to file: %v", err)
		}
		err = os.Rename(tempFile, *auditOutputFile)
		if err != nil {
			logrus.Fatalf("error renaming output file: %v", err)
		}
	}
}

func getKubeClient() (dynamic.Interface, meta.RESTMapper, kubernetes.Interface, string, error) {
	kubeConf, err := ctrl.GetConfig()
	if err != nil {
		return nil, nil, nil, "", fmt.Errorf("fetching KubeConfig: %w", err)
	}

	kube, err := kubernetes.NewForConfig(kubeConf)
	if err != nil {
		return nil, nil, nil, "", fmt.Errorf("creating Kubernetes client: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(kubeConf)
	if err != nil {
		return nil, nil, nil, "", fmt.Errorf("creating Dynamic client: %w", err)
	}

	resources, err := restmapper.GetAPIGroupResources(kube.Discovery())
	if err != nil {
		return nil, nil, nil, "", fmt.Errorf("getting API Group resources: %w", err)
	}
	restMapper := restmapper.NewDiscoveryRESTMapper(resources)
	return dynamicClient, restMapper, kube, kubeConf.Host, nil
}
