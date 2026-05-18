package main

import (
	"context"
	"encoding/json"
	"os"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Client struct {
	RestMapper       meta.RESTMapper
	DynamicInterface dynamic.Interface
	Kube             kubernetes.Interface
}

func main() {
	logrus.Info("Starting Kyverno plugin")
	client, err := getKubeClient()
	if err != nil {
		logrus.Fatal("Error getting kube client: ", err)
	}
	ctx := context.Background()
	kyvernoVersion := detectKyvernoVersion(ctx, client.Kube)
	if kyvernoVersion != "" {
		logrus.Infof("Detected Kyverno version from admission controller image: %s", kyvernoVersion)
	}
	clusterPoliciesTitleAndDescription, err := createPoliciesTitleAndDescriptionMap(ctx, "ClusterPolicy", client)
	if err != nil {
		logrus.Fatal("Error creating policies title and description map: ", err)
	}
	policyReports, err := client.ListResources(ctx, "PolicyReport", client.DynamicInterface, client.RestMapper)
	if err != nil {
		logrus.Fatal("Error listing policy reports: ", err)
	}
	policyReportsTitleAndDescription, err := createPoliciesTitleAndDescriptionMap(ctx, "PolicyReport", client)
	if err != nil {
		logrus.Fatal("Error creating policies title and description map: ", err)
	}
	clusterPolicyReports, err := client.ListResources(context.Background(), "ClusterPolicyReport", client.DynamicInterface, client.RestMapper)
	if err != nil {
		logrus.Fatal("Error listing cluster policy reports: ", err)
	}
	policyReportsViolations, err := filterViolations(policyReports, policyReportsTitleAndDescription)
	if err != nil {
		logrus.Fatal("Error filtering violations: ", err)
	}
	logrus.Info("Policy reports violations found: ", len(policyReportsViolations))
	clusterPolicyReportsViolations, err := filterViolations(clusterPolicyReports, clusterPoliciesTitleAndDescription)
	if err != nil {
		logrus.Fatal("Error filtering violations: ", err)
	}
	logrus.Info("Cluster policy reports violations found: ", len(clusterPolicyReportsViolations))
	validatingAdmissionPolicyReports, err := filterValidationAdmissionPolicyReports(policyReports)
	if err != nil {
		logrus.Fatal("Error filtering validating admission policy reports: ", err)
	}
	logrus.Info("Validating admission policy reports found: ", len(validatingAdmissionPolicyReports))
	validatingAdmissionPolicies, err := client.ListResources(context.Background(), "ValidatingAdmissionPolicy", client.DynamicInterface, client.RestMapper)
	if err != nil {
		logrus.Fatal("Error listing validating admission policies: ", err)
	}
	filteredValidatingAdmissionPolicies, err := removeManagedFields(validatingAdmissionPolicies)
	if err != nil {
		logrus.Fatal("Error filtering validating admission policies: ", err)
	}
	clusterPolicies, err := client.ListResources(context.Background(), "ClusterPolicy", client.DynamicInterface, client.RestMapper)
	if err != nil {
		logrus.Fatal("Error listing cluster policies: ", err)
	}
	filteredClusterPolicies, err := removeManagedFields(clusterPolicies)
	if err != nil {
		logrus.Fatal("Error filtering cluster policies: ", err)
	}
	logrus.Info("Cluster policies found: ", len(filteredClusterPolicies))

	logrus.Info("Validating admission policies found: ", len(filteredValidatingAdmissionPolicies))
	response := map[string]any{
		"kyvernoVersion":                   kyvernoVersion,
		"policyReports":                    policyReportsViolations,
		"clusterPolicyReports":             clusterPolicyReportsViolations,
		"validatingAdmissionPolicyReports": validatingAdmissionPolicyReports,
		"validatingAdmissionPolicies":      filteredValidatingAdmissionPolicies,
		"clusterPolicies":                  filteredClusterPolicies,
	}
	jsonBytes, err := json.Marshal(response)
	if err != nil {
		logrus.Fatal("Error marshalling response: ", err)
	}
	logrus.Info("Writing Kyverno plugin output to /output/kyverno.json")
	err = os.WriteFile("/output/kyverno-temp.json", jsonBytes, 0644)
	if err != nil {
		logrus.Fatal("Error writing output file: ", err)
	}
	err = os.Rename("/output/kyverno-temp.json", "/output/kyverno.json")
	if err != nil {
		logrus.Fatal("Error renaming output file: ", err)
	}
	logrus.Info("Kyverno plugin finished")
}

func filterViolations(policies []unstructured.Unstructured, policiesTitleAndDDescription map[string]any) ([]map[string]any, error) {
	allViolations := []map[string]any{}
	for _, p := range policies {
		metadata := p.Object["metadata"].(map[string]any)
		delete(metadata, "managedFields")
		results := p.Object["results"].([]any)
		violations := []map[string]any{}
		for _, r := range results {
			result := r.(map[string]any)
			if result["result"].(string) != "fail" && result["result"].(string) != "warn" {
				continue
			}
			if titleAndDescription, ok := policiesTitleAndDDescription[result["policy"].(string)]; ok {
				result["policyTitle"] = titleAndDescription.(map[string]any)["title"]
				result["policyDescription"] = titleAndDescription.(map[string]any)["description"]
			}
			violations = append(violations, result)
		}
		if len(violations) == 0 {
			continue
		}
		p.Object["results"] = violations
		allViolations = append(allViolations, p.Object)
	}
	return allViolations, nil
}

func filterValidationAdmissionPolicyReports(policies []unstructured.Unstructured) ([]map[string]any, error) {
	result := []map[string]any{}
	for _, p := range policies {
		metadata := p.Object["metadata"].(map[string]any)
		delete(metadata, "managedFields")
		if _, ok := p.Object["results"].([]any); ok {
			results := p.Object["results"].([]any)
			violations := []map[string]any{}
			for _, r := range results {
				result := r.(map[string]any)
				if result["result"] == nil || result["source"] != "ValidatingAdmissionPolicy" {
					continue
				}
				violations = append(violations, result)
			}
			if len(violations) == 0 {
				continue
			}
			p.Object["results"] = violations
			result = append(result, p.Object)
			continue
		}
		if _, ok := p.Object["results"].([]map[string]any); !ok {
			results := p.Object["results"].([]map[string]any)
			violations := []map[string]any{}
			for _, r := range results {
				result := r
				if result["result"] == nil || result["source"] != "ValidatingAdmissionPolicy" {
					continue
				}
				violations = append(violations, result)
			}
			if len(violations) == 0 {
				continue
			}
			p.Object["results"] = violations
			result = append(result, p.Object)
			continue
		}

	}
	return result, nil
}

func removeManagedFields(policies []unstructured.Unstructured) ([]map[string]any, error) {
	result := []map[string]any{}
	for _, p := range policies {
		if _, ok := p.Object["metadata"].(map[string]any); ok {
			metadata := p.Object["metadata"].(map[string]any)
			delete(metadata, "managedFields")
		}
		result = append(result, p.Object)
	}
	return result, nil
}

func createPoliciesTitleAndDescriptionMap(ctx context.Context, resourceType string, client *Client) (map[string]any, error) {
	clusterPoliciesMetadata, err := client.ListResources(ctx, resourceType, client.DynamicInterface, client.RestMapper)
	if err != nil {
		return nil, err
	}
	policiesTitleAndDDescription := map[string]any{}
	for _, p := range clusterPoliciesMetadata {
		metadata := p.Object["metadata"].(map[string]any)
		if annotations, ok := metadata["annotations"]; ok {
			annotationsMap := annotations.(map[string]any)
			title := ""
			description := ""
			if annotationsMap["policies.kyverno.io/title"] != nil {
				title = annotationsMap["policies.kyverno.io/title"].(string)
			}
			if annotationsMap["policies.kyverno.io/description"] != nil {
				description = annotationsMap["policies.kyverno.io/description"].(string)
			}
			policiesTitleAndDDescription[p.GetName()] = map[string]any{
				"title":       title,
				"description": description,
			}

		}
	}
	return policiesTitleAndDDescription, nil
}

func getKubeClient() (*Client, error) {
	config, err := ctrl.GetConfig()
	if err != nil {
		return nil, err
	}
	dynamicInterface, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	kube, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	groupResources, err := restmapper.GetAPIGroupResources(kube.Discovery())
	if err != nil {
		return nil, err
	}
	restMapper := restmapper.NewDiscoveryRESTMapper(groupResources)

	client := Client{
		RestMapper:       restMapper,
		DynamicInterface: dynamicInterface,
		Kube:             kube,
	}
	return &client, nil
}

// detectKyvernoVersion returns the admission-controller container image tag
// (e.g. v1.18.0) when a standard Kyverno Helm install is present; otherwise "".
func detectKyvernoVersion(ctx context.Context, kube kubernetes.Interface) string {
	for _, ns := range []string{"kyverno", "kyverno-system"} {
		if v := detectKyvernoVersionInNamespace(ctx, kube, ns); v != "" {
			return v
		}
	}
	return ""
}

func detectKyvernoVersionInNamespace(ctx context.Context, kube kubernetes.Interface, namespace string) string {
	deploys, err := kube.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/component=admission-controller",
	})
	if err != nil {
		logrus.Debugf("listing admission-controller deployments in namespace %q: %v", namespace, err)
		return ""
	}
	if len(deploys.Items) == 0 {
		return ""
	}
	items := deploys.Items
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	for i := range items {
		if v := kyvernoVersionFromContainers(items[i].Spec.Template.Spec.Containers); v != "" {
			return v
		}
	}
	return ""
}

// kyvernoVersionFromContainers returns an image tag from the main Kyverno workload
// container, avoiding unrelated sidecars that also use tagged images.
func kyvernoVersionFromContainers(containers []corev1.Container) string {
	for _, c := range containers {
		if c.Name != "kyverno" {
			continue
		}
		if tag := extractVersionFromImage(c.Image); tag != "" {
			return tag
		}
	}
	for _, c := range containers {
		if !strings.Contains(strings.ToLower(imageRefWithoutDigest(c.Image)), "kyverno/kyverno") {
			continue
		}
		if tag := extractVersionFromImage(c.Image); tag != "" {
			return tag
		}
	}
	return ""
}

func imageRefWithoutDigest(image string) string {
	image = strings.TrimSpace(image)
	if i := strings.Index(image, "@"); i >= 0 {
		return image[:i]
	}
	return image
}

func extractVersionFromImage(image string) string {
	image = imageRefWithoutDigest(image)
	if image == "" {
		return ""
	}
	idx := strings.LastIndex(image, ":")
	if idx < 0 || idx >= len(image)-1 {
		return ""
	}
	return strings.TrimSpace(image[idx+1:])
}

func (c *Client) ListResources(ctx context.Context, resourceType string, dynamicClient dynamic.Interface, restMapper meta.RESTMapper) ([]unstructured.Unstructured, error) {
	gvr, err := restMapper.ResourceFor(schema.GroupVersionResource{
		Resource: resourceType,
	})
	if err != nil {
		return nil, err
	}
	list, err := dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}
