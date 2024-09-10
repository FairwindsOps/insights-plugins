package main

import (
	"context"
	"encoding/json"
	"os"

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
}

func main() {
	client, err := getKubeClient()
	if err != nil {
		panic(err)
	}
	policiesTitleAndDDescription, err := createPoliciesTitleAndDescriptionMap(client)
	if err != nil {
		panic(err)
	}
	policyReports, err := client.ListPolicies(context.Background(), "PolicyReport", client.DynamicInterface, client.RestMapper)
	if err != nil {
		panic(err)
	}
	clusterPolicyReports, err := client.ListPolicies(context.Background(), "ClusterPolicyReport", client.DynamicInterface, client.RestMapper)
	if err != nil {
		panic(err)
	}
	policyReportsViolations, err := getViolations(policyReports, policiesTitleAndDDescription)
	if err != nil {
		panic(err)
	}
	clusterPolicyReportsViolations, err := getViolations(clusterPolicyReports, policiesTitleAndDDescription)
	if err != nil {
		panic(err)
	}
	response := map[string]interface{}{
		"policyReports":        policyReportsViolations,
		"clusterPolicyReports": clusterPolicyReportsViolations,
	}
	jsonBytes, err := json.Marshal(response)
	if err != nil {
		panic(err)
	}
	err = os.WriteFile("/output/kyverno.json", jsonBytes, 0644)
	if err != nil {
		panic(err)
	}
}

func getViolations(policies []unstructured.Unstructured, policiesTitleAndDDescription map[string]interface{}) ([]map[string]interface{}, error) {
	allViolations := []map[string]interface{}{}
	for _, p := range policies {
		metadata := p.Object["metadata"].(map[string]interface{})
		delete(metadata, "managedFields")
		results := p.Object["results"].([]interface{})
		violations := []map[string]interface{}{}
		for _, r := range results {
			result := r.(map[string]interface{})
			if result["result"].(string) != "fail" && result["result"].(string) != "warn" {
				continue
			}
			if titleAndDescription, ok := policiesTitleAndDDescription[result["policy"].(string)]; ok {
				result["policyTitle"] = titleAndDescription.(map[string]interface{})["title"]
				result["policyDescription"] = titleAndDescription.(map[string]interface{})["description"]
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

func createPoliciesTitleAndDescriptionMap(client *Client) (map[string]interface{}, error) {
	clusterPoliciesMetadata, err := client.ListPolicies(context.Background(), "ClusterPolicy", client.DynamicInterface, client.RestMapper)
	if err != nil {
		return map[string]interface{}{}, err
	}
	policiesTitleAndDDescription := map[string]interface{}{}
	for _, p := range clusterPoliciesMetadata {
		metadata := p.Object["metadata"].(map[string]interface{})
		if annotations, ok := metadata["annotations"]; ok {
			annotationsMap := annotations.(map[string]interface{})
			title := ""
			description := ""
			if annotationsMap["policies.kyverno.io/title"] != nil {
				title = annotationsMap["policies.kyverno.io/title"].(string)
			}
			if annotationsMap["policies.kyverno.io/description"] != nil {
				description = annotationsMap["policies.kyverno.io/description"].(string)
			}
			policiesTitleAndDDescription[p.GetName()] = map[string]interface{}{
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
		restMapper,
		dynamicInterface,
	}
	return &client, nil
}

func (c *Client) ListPolicies(ctx context.Context, resourceType string, dynamicClient dynamic.Interface, restMapper meta.RESTMapper) ([]unstructured.Unstructured, error) {
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
