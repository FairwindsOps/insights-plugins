package main

import (
	"context"
	"encoding/json"
	"flag"
	"os"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const tempFile = "/output/rbac-reporter-temp.json"

func main() {
	auditOutputFile := flag.String("output-file", "", "Destination file for audit results")
	flag.Parse()

	ctrl.SetLogger(zap.New(func(o *zap.Options) {
		o.Development = true
	}))

	kubeConf, configError := ctrl.GetConfig()
	if configError != nil {
		logrus.Fatalf("Error fetching KubeConfig %v", configError)
	}

	api, err := kubernetes.NewForConfig(kubeConf)
	if err != nil {
		logrus.Fatalf("Error creating Kubernetes client %v", err)
	}
	logrus.Info("connected to kube")

	resources, err := CreateResourceProviderFromAPI(context.Background(), api, kubeConf.Host)

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
		err := os.WriteFile(tempFile, outputBytes, 0644)
		if err != nil {
			logrus.Fatalf("Error writing output to file: %v", err)
		}
		err = os.Rename(tempFile, *auditOutputFile)
		if err != nil {
			logrus.Fatalf("Error renaming output file: %v", err)
		}
	}
}
