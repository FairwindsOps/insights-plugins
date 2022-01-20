package opa

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/fairwindsops/insights-plugins/opa/pkg/kube"
	"github.com/fairwindsops/insights-plugins/opa/pkg/rego"
)

var (
	defaultSeverity    = 0.5
	defaultTitle       = "Unknown title"
	defaultDescription = ""
	defaultRemediation = ""
	defaultCategory    = "Security"
)

var instanceGvr = schema.GroupVersionResource{Group: "insights.fairwinds.com", Version: "v1beta1", Resource: "customcheckinstances"}
var checkGvr = schema.GroupVersionResource{Group: "insights.fairwinds.com", Version: "v1beta1", Resource: "customchecks"}

func Run(ctx context.Context) ([]ActionItem, error) {
	thisNamespace := "insights-agent"
	namespaceBytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err == nil { //Ignore errors because that means this isn't running in a container
		thisNamespace = string(namespaceBytes)
	}

	client := kube.GetKubeClient()
	checkInstances, err := client.DynamicInterface.Resource(instanceGvr).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	logrus.Infof("Found %d checks", len(checkInstances.Items))

	ais, err := processAllChecks(ctx, client, checkInstances.Items, thisNamespace)
	if err != nil {
		return nil, err
	}
	return ais, nil
}

func processAllChecks(ctx context.Context, client *kube.Client, checkInstances []unstructured.Unstructured, thisNamespace string) ([]ActionItem, error) {
	actionItems := make([]ActionItem, 0)
	var lastError error = nil

	for _, checkInstance := range checkInstances {
		if checkInstance.GetLabels()["insights.fairwinds.com/managed"] != "" && checkInstance.GetNamespace() != thisNamespace {
			continue
		}
		var checkInstanceObject CustomCheckInstance
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(checkInstance.Object, &checkInstanceObject)
		if err != nil {
			lastError = fmt.Errorf("failed to parse a check instance: %v", err)
			logrus.Warn(lastError.Error())
			continue
		}
		logrus.Infof("Starting to process check: %s", checkInstanceObject.Name)
		check, err := client.DynamicInterface.Resource(checkGvr).Namespace(checkInstanceObject.Namespace).Get(ctx, checkInstanceObject.Spec.CustomCheckName, metav1.GetOptions{})
		if err != nil {
			lastError = fmt.Errorf("failed to find check %s/%s referenced by instance %s: %v", checkInstanceObject.Namespace, checkInstanceObject.Spec.CustomCheckName, checkInstanceObject.Name, err)
			logrus.Warn(lastError.Error())
			continue
		}
		var checkObject CustomCheck
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(check.Object, &checkObject)
		if err != nil {
			lastError = fmt.Errorf("failed to parse check %s/%s: %v", checkInstanceObject.Namespace, checkInstanceObject.Spec.CustomCheckName, err)
			logrus.Warn(lastError.Error())
			continue
		}

		newItems, err := processCheck(ctx, checkObject, checkInstanceObject)
		if err != nil {
			lastError = fmt.Errorf("error while processing check instance %s/%s: %v", checkInstanceObject.Namespace, checkInstanceObject.Name, err)
			logrus.Warn(lastError.Error())
			continue
		}
		actionItems = append(actionItems, newItems...)
	}
	return actionItems, lastError
}

func processCheck(ctx context.Context, check CustomCheck, checkInstance CustomCheckInstance) ([]ActionItem, error) {
	actionItems := make([]ActionItem, 0)

	for _, gk := range getGroupKinds(checkInstance.Spec.Targets) {
		newAI, err := processCheckTarget(ctx, check, checkInstance, gk)
		if err != nil {
			logrus.Errorf("Error while processing check %s: %v", checkInstance.Spec.CustomCheckName, err)
			return nil, err
		}

		actionItems = append(actionItems, newAI...)
	}

	return actionItems, nil
}

func processCheckTarget(ctx context.Context, check CustomCheck, checkInstance CustomCheckInstance, gk schema.GroupKind) ([]ActionItem, error) {
	client := kube.GetKubeClient()
	actionItems := make([]ActionItem, 0)
	mapping, err := client.RestMapper.RESTMapping(gk)
	if err != nil {
		return actionItems, err
	}
	gvr := mapping.Resource
	list, err := client.DynamicInterface.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, obj := range list.Items {
		newItems, err := ProcessCheckForItem(ctx, check, checkInstance, obj.Object, obj.GetName(), obj.GetKind(), obj.GetNamespace())
		if err != nil {
			return nil, err
		}
		actionItems = append(actionItems, newItems...)
	}
	return actionItems, nil
}

func ProcessCheckForItem(ctx context.Context, check CustomCheck, instance CustomCheckInstance, obj map[string]interface{}, resourceName, resourceKind, resourceNamespace string) ([]ActionItem, error) {
	results, err := runRegoForItem(ctx, check.Spec.Rego, instance.Spec.Parameters, obj)
	if err != nil {
		logrus.Errorf("Error while running rego for item %s/%s/%s: %v", resourceKind, resourceNamespace, resourceName, err)
		return nil, err
	}
	aiDetails := OutputFormat{}
	aiDetails.SetDefaults(check.Spec.Output, instance.Spec.Output)
	newItems, err := processResults(resourceName, resourceKind, resourceNamespace, results, instance.Name, aiDetails)
	return newItems, err
}

func runRegoForItem(ctx context.Context, body string, params map[string]interface{}, obj map[string]interface{}) ([]interface{}, error) {
	client := kube.GetKubeClient()
	return rego.RunRegoForItem(ctx, body, params, obj, *client)
}

func getInsightsChecks() (clusterCheckModel, error) {
	var jsonResponse clusterCheckModel

	url := os.Getenv("FAIRWINDS_INSIGHTS_HOST") + "/v0/organizations/" + os.Getenv("FAIRWINDS_ORG") + "/clusters/" + os.Getenv("FAIRWINDS_CLUSTER") +
		"/data/opa/customChecks"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return jsonResponse, err
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("FAIRWINDS_TOKEN"))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return jsonResponse, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return jsonResponse, fmt.Errorf("failed to retrieve updated checks with a status code of : %d", resp.StatusCode)
	}
	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return jsonResponse, err
	}
	err = json.Unmarshal(responseBody, &jsonResponse)
	if err != nil {
		return jsonResponse, err
	}
	return jsonResponse, nil
}

func maybeGetStringField(m map[string]interface{}, key string) (*string, error) {
	if m[key] == nil {
		return nil, nil
	}
	str, ok := m[key].(string)
	if !ok {
		return nil, errors.New(key + " was not a string")
	}
	return &str, nil
}

func maybeGetFloatField(m map[string]interface{}, key string) (*float64, error) {
	if m[key] == nil {
		return nil, nil
	}
	n, ok := m[key].(json.Number)
	if !ok {
		return nil, errors.New(key + " was not a float")
	}
	f, err := n.Float64()
	if err != nil {
		return nil, err
	}
	return &f, nil
}

func getDetailsFromMap(m map[string]interface{}) (OutputFormat, error) {
	output := OutputFormat{}
	var err error
	output.Description, err = maybeGetStringField(m, "description")
	if err != nil {
		return output, err
	}
	output.Title, err = maybeGetStringField(m, "title")
	if err != nil {
		return output, err
	}
	output.Category, err = maybeGetStringField(m, "category")
	if err != nil {
		return output, err
	}
	output.Remediation, err = maybeGetStringField(m, "remediation")
	if err != nil {
		return output, err
	}
	output.Severity, err = maybeGetFloatField(m, "severity")
	if err != nil {
		return output, err
	}
	return output, nil
}

func processResults(resourceName, resourceKind, resourceNamespace string, results []interface{}, name string, details OutputFormat) ([]ActionItem, error) {
	actionItems := make([]ActionItem, 0)
	for _, output := range results {
		strMethod, ok := output.(string)
		outputDetails := OutputFormat{}
		if ok {
			outputDetails.Description = &strMethod
		} else {
			mapMethod, ok := output.(map[string]interface{})
			if ok {
				var err error
				outputDetails, err = getDetailsFromMap(mapMethod)
				if err != nil {
					return nil, err
				}
			} else {
				return nil, fmt.Errorf("could not decipher output format of %+v %T", output, output)
			}
		}
		outputDetails.SetDefaults(details, OutputFormat{
			Severity:    &defaultSeverity,
			Title:       &defaultTitle,
			Remediation: &defaultRemediation,
			Category:    &defaultCategory,
			Description: &defaultDescription,
		})

		actionItems = append(actionItems, ActionItem{
			EventType:         name,
			ResourceNamespace: resourceNamespace,
			ResourceKind:      resourceKind,
			ResourceName:      resourceName,
			Title:             *outputDetails.Title,
			Description:       *outputDetails.Description,
			Remediation:       *outputDetails.Remediation,
			Severity:          *outputDetails.Severity,
			Category:          *outputDetails.Category,
		})
	}

	return actionItems, nil
}

func getGroupKinds(targets []KubeTarget) []schema.GroupKind {
	kinds := make([]schema.GroupKind, 0)
	for _, target := range targets {
		for _, apiGroup := range target.APIGroups {
			for _, kind := range target.Kinds {
				kinds = append(kinds, schema.GroupKind{Group: apiGroup, Kind: kind})
			}
		}
	}
	return kinds
}
