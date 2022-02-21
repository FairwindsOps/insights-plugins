package opa

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/fairwindsops/insights-plugins/opa/pkg/kube"
	"github.com/fairwindsops/insights-plugins/opa/pkg/rego"
	"github.com/hashicorp/go-multierror"
)

var (
	defaultSeverity    = 0.5
	defaultTitle       = "Unknown title"
	defaultDescription = ""
	defaultRemediation = ""
	defaultCategory    = "Security"
	CLIKubeTargets     *[]KubeResourceTarget
)

var instanceGvr = schema.GroupVersionResource{Group: "insights.fairwinds.com", Version: "v1beta1", Resource: "customcheckinstances"}
var checkGvr = schema.GroupVersionResource{Group: "insights.fairwinds.com", Version: "v1beta1", Resource: "customchecks"}

func Run(ctx context.Context) ([]ActionItem, error) {
	// Temporary until CLI processing happens.
	/*
	   CLIKubeTargets = make([]KubeResourceTarget, 2)
	   	CLIKubeTargets[0].APIGroups = make([]string, 1)
	   	CLIKubeTargets[0].Resources = make([]string, 2)
	*/
	/*
	   convertedGK, err := GroupResourceToGroupKind(schema.GroupResource{Group: "apps", Resource: "Deployments"})
	   	if err != nil {
	   		fmt.Printf("Error converting GR to GK: %v\n", err)
	   	}
	   	CLIKubeTargets[0].APIGroups[0] = convertedGK.Group
	   	CLIKubeTargets[0].Kinds[0] = convertedGK.Kind
	*/
	/*
	   CLIKubeTargets[0].APIGroups[0] = "apps"
	   	CLIKubeTargets[0].Resources[0] = "Deployments"
	   	CLIKubeTargets[0].Resources[1] = "StatefulSets"
	   	CLIKubeTargets[1].APIGroups = make([]string, 1)
	   	CLIKubeTargets[1].Resources = make([]string, 1)
	   	CLIKubeTargets[1].APIGroups[0] = "batch"
	   	CLIKubeTargets[1].Resources[0] = "CronJobs"
	*/
	// CLIKubeTargets = ProcessCLIKubeResourceTargets(viper.GetStringSlice("target-resource"))

	thisNamespace := "insights-agent"
	namespaceBytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err == nil { //Ignore errors because that means this isn't running in a container
		thisNamespace = string(namespaceBytes)
	}

	jsonResponse, err := getInsightsChecks()
	if err != nil {
		return nil, err
	}

	logrus.Infof("Found %d checks and %d instances", len(jsonResponse.Checks), len(jsonResponse.Instances))

	ais, err := processAllChecks(ctx, jsonResponse.Instances, jsonResponse.Checks, thisNamespace)
	if err != nil {
		return nil, err
	}
	return ais, nil
}

func processAllChecks(ctx context.Context, checkInstances []CheckSetting, checks []OPACustomCheck, thisNamespace string) ([]ActionItem, error) {
	actionItems := make([]ActionItem, 0)
	var lastError error = nil
	var allErrs error = nil

	for _, check := range checks {
		logrus.Debugf("Check %s is version %.1f", check.Name, check.Version)
		switch check.Version {
		case 1.0:
			for _, checkInstance := range checkInstances {
				if check.Name == checkInstance.CheckName {
					logrus.Debugf("Found instance %s to match check %s", checkInstance.AdditionalData.Name, check.Name)
					newItems, err := processCheck(ctx, check, checkInstance.GetCustomCheckInstance())
					if err != nil {
						lastError = fmt.Errorf("error while processing check %s / instance %s: %v", check.Name, checkInstance.GetCustomCheckInstance().Name, err)
						allErrs = multierror.Append(allErrs, lastError)
						break // Go back to outer checks loop
					}
					actionItems = append(actionItems, newItems...)
					break // Go back to outer checks loop
				}
			}
		case 2.0:
			newItems, err := processCheckV2(ctx, check)
			if err != nil {
				lastError = fmt.Errorf("error while processing check %s: %v", check.Name, err)
				allErrs = multierror.Append(allErrs, lastError)
			}
			actionItems = append(actionItems, newItems...)
		default:
			lastError = fmt.Errorf("CustomCheck %s is an unexpected version %.1f and will not be run", check.Name, check.Version)
			allErrs = multierror.Append(allErrs, lastError)
		}
	}
	return actionItems, allErrs
}

func processCheck(ctx context.Context, check OPACustomCheck, checkInstance CustomCheckInstance) ([]ActionItem, error) {
	actionItems := make([]ActionItem, 0)
	var allErrs error = nil

	for _, gk := range getGroupKinds(checkInstance.Spec.Targets) {
		newAI, err := processCheckTarget(ctx, check, checkInstance, gk)
		if err != nil {
			allErrs = multierror.Append(allErrs, err)
		}

		actionItems = append(actionItems, newAI...)
	}

	return actionItems, multierror.Prefix(allErrs, " ")
}

func processCheckV2(ctx context.Context, check OPACustomCheck) ([]ActionItem, error) {
	actionItems := make([]ActionItem, 0)
	var allErrs error = nil

	for _, gr := range getGroupResources(*CLIKubeTargets) {
		newAI, err := processCheckTargetV2(ctx, check, gr)
		if err != nil {
			allErrs = multierror.Append(allErrs, err)
		}

		actionItems = append(actionItems, newAI...)
	}

	return actionItems, multierror.Prefix(allErrs, " ")
}

func processCheckTarget(ctx context.Context, check OPACustomCheck, checkInstance CustomCheckInstance, gk schema.GroupKind) ([]ActionItem, error) {
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
		newItems, err := ProcessCheckForItem(ctx, check, checkInstance, obj.Object, obj.GetName(), obj.GetKind(), obj.GetNamespace(), &rego.InsightsInfo{InsightsContext: "Agent", Cluster: os.Getenv("FAIRWINDS_CLUSTER")})
		if err != nil {
			return nil, err
		}
		actionItems = append(actionItems, newItems...)
	}
	return actionItems, nil
}

func processCheckTargetV2(ctx context.Context, check OPACustomCheck, gr schema.GroupResource) ([]ActionItem, error) {
	client := kube.GetKubeClient()
	actionItems := make([]ActionItem, 0)
	gvr, err := client.RestMapper.ResourceFor(gr.WithVersion(""))
	if err != nil {
		return nil, fmt.Errorf("error getting GroupVersionResource from GroupResource %q: %v", gr, err)
	}
	list, err := client.DynamicInterface.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listing %q for check %s and target %s: %v", gvr, check.Name, gr, err)
	}
	logrus.Debugf("Listed %d %q objects for check %s, target %s", len(list.Items), gvr, check.Name, gr)
	for _, obj := range list.Items {
		newItems, err := ProcessCheckForItemV2(ctx, check, obj.Object, obj.GetName(), obj.GetKind(), obj.GetNamespace(), &rego.InsightsInfo{InsightsContext: "Agent", Cluster: os.Getenv("FAIRWINDS_CLUSTER")})
		if err != nil {
			return nil, err
		}
		actionItems = append(actionItems, newItems...)
	}
	return actionItems, nil
}

func ProcessCheckForItem(ctx context.Context, check OPACustomCheck, instance CustomCheckInstance, obj map[string]interface{}, resourceName, resourceKind, resourceNamespace string, insightsInfo *rego.InsightsInfo) ([]ActionItem, error) {
	results, err := runRegoForItem(ctx, check.Rego, instance.Spec.Parameters, obj, insightsInfo)
	if err != nil {
		return nil, fmt.Errorf("error while running rego for check %s on item %s/%s/%s: %v", check.Name, resourceKind, resourceNamespace, resourceName, err)
	}
	aiDetails := OutputFormat{}
	aiDetails.SetDefaults(check.GetOutputFormat(), instance.Spec.Output)
	newItems, err := processResults(resourceName, resourceKind, resourceNamespace, results, check.Name, aiDetails)
	return newItems, err
}

func ProcessCheckForItemV2(ctx context.Context, check OPACustomCheck, obj map[string]interface{}, resourceName, resourceKind, resourceNamespace string, insightsInfo *rego.InsightsInfo) ([]ActionItem, error) {
	results, err := runRegoForItemV2(ctx, check.Rego, obj, insightsInfo)
	if err != nil {
		return nil, fmt.Errorf("error while running rego for check %s on item %s/%s/%s: %v", check.Name, resourceKind, resourceNamespace, resourceName, err)
	}
	aiDetails := OutputFormat{}
	aiDetails.SetDefaults(check.GetOutputFormat())
	newItems, err := processResults(resourceName, resourceKind, resourceNamespace, results, check.Name, aiDetails)
	return newItems, err
}

func runRegoForItem(ctx context.Context, body string, params map[string]interface{}, obj map[string]interface{}, insightsInfo *rego.InsightsInfo) ([]interface{}, error) {
	client := kube.GetKubeClient()
	return rego.RunRegoForItem(ctx, body, params, obj, *client, insightsInfo)
}

func runRegoForItemV2(ctx context.Context, body string, obj map[string]interface{}, insightsInfo *rego.InsightsInfo) ([]interface{}, error) {
	client := kube.GetKubeClient()
	return rego.RunRegoForItemV2(ctx, body, obj, *client, insightsInfo)
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

func getGroupResources(targets []KubeResourceTarget) []schema.GroupResource {
	GRs := make([]schema.GroupResource, 0)
	for _, target := range targets {
		for _, apiGroup := range target.APIGroups {
			for _, resource := range target.Resources {
				GRs = append(GRs, schema.GroupResource{Group: apiGroup, Resource: resource})
			}
		}
	}
	return GRs
}

// ProcessClIKubeResourceTargets processes command-line flag input and returns
// a slice of KubeResourceTarget.
func ProcessCLIKubeResourceTargets(CLIInputs []string) *[]KubeResourceTarget {
	if CLIInputs == nil {
		return nil
	}
	targets := make([]KubeResourceTarget, 0)
	for _, CLIInput := range CLIInputs {
		fields := strings.Split(CLIInput, "/")
		var target KubeResourceTarget
		target.APIGroups = append(target.APIGroups, strings.Split(fields[0], ",")...)
		target.Resources = append(target.Resources, strings.Split(fields[1], ",")...)
		if len(target.APIGroups) > 0 || len(target.Resources) > 0 {
			targets = append(targets, target)
		}
	}
	logrus.Debugf("Parsed command-line options %q into %d target resources: %#v", CLIInputs, len(targets), targets)
	return &targets
}
