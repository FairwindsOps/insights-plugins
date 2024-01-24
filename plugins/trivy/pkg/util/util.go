package util

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/fairwindsops/insights-plugins/plugins/trivy/pkg/models"
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
const DockerIOprefix = "docker.io/"

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

func CheckEnvironmentVariables() error {
	if os.Getenv("FAIRWINDS_INSIGHTS_HOST") == "" || os.Getenv("FAIRWINDS_ORG") == "" || os.Getenv("FAIRWINDS_CLUSTER") == "" || os.Getenv("FAIRWINDS_TOKEN") == "" {
		return errors.New("Proper environment variables not set.")
	}
	return nil
}

func RunCommand(cmd *exec.Cmd, message string) error {
	logrus.Info(message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputString := string(output)
		logrus.Errorf("Error %s: %s\n%s", message, err, outputString)
		if strings.Contains(outputString, UnknownOSMessage) {
			return errors.New(UnknownOSMessage)
		}
	}
	return err
}

func UnmarshalAndFixReport(body []byte) (*models.MinimizedReport, error) {
	var report models.MinimizedReport
	err := json.Unmarshal(body, &report)
	if err != nil {
		return nil, err
	}
	fixOwners(&report)
	normalizeDockerHubImages(&report)
	return &report, nil
}

// fixOwners adapt older owners fields to the new ones
func fixOwners(report *models.MinimizedReport) {
	for i := range report.Images {
		img := &report.Images[i]
		if hasDeprecatedOwnerFields(*img) {
			var container string
			if img.OwnerContainer != nil {
				container = *img.OwnerContainer
			}
			img.Owners = []models.Resource{
				{
					Name:      img.OwnerName,
					Kind:      img.OwnerKind,
					Namespace: img.Namespace,
					Container: container,
				},
			}
		}
	}
}

func hasDeprecatedOwnerFields(img models.ImageDetailsWithRefs) bool {
	return len(img.OwnerName) != 0 || len(img.OwnerKind) != 0 || len(img.Namespace) != 0
}

// normalizeDockerHubImages removes the docker.io/ prefix from the image names and IDs
func normalizeDockerHubImages(report *models.MinimizedReport) {
	for i := range report.Images {
		img := &report.Images[i]
		img.Name = strings.TrimPrefix(img.Name, DockerIOprefix)
		img.ID = strings.TrimPrefix(img.ID, DockerIOprefix)
	}
}
