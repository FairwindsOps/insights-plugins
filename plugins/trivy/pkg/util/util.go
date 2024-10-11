package util

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const UnknownOSMessage = "Unknown OS"

// KubeClientResources bundles together Kubernetes clients and related
// resources.
type KubeClientResources struct {
	Client        kubernetes.Interface
	DynamicClient dynamic.Interface // used to find owning pod-controller
	RESTMapper    meta.RESTMapper   // used with dynamicClient
}

// CreateKubeClientResources returns a KubeClientResources type, trying first
// in-cluster, then local, KubeConfig.
func CreateKubeClientResources() KubeClientResources {
	// Try to load in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		logrus.Infof("Could not load in-cluster config, falling back to $KUBECONFIG or ~/.kube/config: %v", err)
		var kubeConfigFilePath string
		kubeConfigFilePath = os.Getenv("KUBECONFIG")
		if kubeConfigFilePath == "" {
			kubeConfigFilePath = homedir.HomeDir() + "/.kube/config"
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeConfigFilePath)
		if err != nil {
			logrus.Fatalf("Failed to load client config %q: %v", kubeConfigFilePath, err)
		}
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		logrus.Fatalf("Failed to create kubernetes client: %v", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		logrus.Fatalf("Error creating dynamic kubernetes client: %v", err)
	}

	resources, err := restmapper.GetAPIGroupResources(client.Discovery())
	if err != nil {
		logrus.Fatalf("Error getting API Group resources: %v", err)
	}

	RESTMapper := restmapper.NewDiscoveryRESTMapper(resources)

	r := KubeClientResources{
		Client:        client,
		DynamicClient: dynamicClient,
		RESTMapper:    RESTMapper,
	}
	return r
}

var requiredEnvVars = []string{"FAIRWINDS_INSIGHTS_HOST", "FAIRWINDS_ORG", "FAIRWINDS_CLUSTER", "FAIRWINDS_TOKEN"}

func CheckEnvironmentVariables() error {
	for _, env := range requiredEnvVars {
		if os.Getenv(env) == "" {
			return fmt.Errorf("missing required environment variable: %s", env)
		}
	}
	return nil
}

func RunCommand(cmd *exec.Cmd, message string) error {
	logrus.Info(message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputString := string(output)
		if strings.Contains(outputString, UnknownOSMessage) {
			return errors.New(UnknownOSMessage)
		}
		return fmt.Errorf("error %s: %s\n%s", message, err, outputString)
	}
	return nil
}
