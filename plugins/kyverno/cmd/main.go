package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	ctrl "sigs.k8s.io/controller-runtime"
)

func main() {
	client, err := getKubeClient()
	if err != nil {
		panic(err)
	}
	policyReports, err := client.ListObjects(context.Background(), "PolicyReport", client.DynamicInterface, client.RestMapper)
	if err != nil {
		panic(err)
	}
	clusterPolicyReports, err := client.ListObjects(context.Background(), "ClusterPolicyReport", client.DynamicInterface, client.RestMapper)
	if err != nil {
		panic(err)
	}

	clusterPoliciesMetadata, err := client.ListObjects(context.Background(), "ClusterPolicy", client.DynamicInterface, client.RestMapper)
	if err != nil {
		panic(err)
	}

	policiesTitleAndDDescription := map[string]interface{}{}
	for _, p := range clusterPoliciesMetadata {
		x := p.Object
		metadata := x["metadata"].(map[string]interface{})
		annotations := metadata["annotations"]
		if annotations != nil {
			annotationsMap := annotations.(map[string]interface{})
			title := annotationsMap["policies.kyverno.io/title"]
			description := annotationsMap["policies.kyverno.io/description"]
			if title != nil && description != nil {
				policiesTitleAndDDescription[p.GetName()] = map[string]interface{}{
					"title":       title,
					"description": description,
				}
			}
		}
	}

	var allPolicyReports []unstructured.Unstructured
	allPolicyReports = append(allPolicyReports, policyReports...)
	allPolicyReports = append(allPolicyReports, clusterPolicyReports...)
	var allViolations []map[string]interface{}
	for _, p := range allPolicyReports {
		metadata := p.Object["metadata"].(map[string]interface{})
		delete(metadata, "managedFields")
		results := p.Object["results"].([]interface{})
		violations := []map[string]interface{}{}
		for _, r := range results {
			result := r.(map[string]interface{})
			if result["result"].(string) != "fail" && result["result"].(string) != "warn" {
				continue
			}
			fmt.Println("X====", result["policy"].(string))
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
	// convert to json
	jsonBytes, err := json.Marshal(allViolations)
	if err != nil {
		panic(err)
	}
	logrus.Infof("Results: %v", string(jsonBytes))
}

type Client struct {
	RestMapper       meta.RESTMapper
	DynamicInterface dynamic.Interface
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

func (c *Client) ListObjects(ctx context.Context, resourceType string, dynamicClient dynamic.Interface, restMapper meta.RESTMapper) ([]unstructured.Unstructured, error) {
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
