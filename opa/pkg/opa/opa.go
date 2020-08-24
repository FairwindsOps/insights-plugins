package opa

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"

	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/types"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/fairwindsops/insights-plugins/opa/pkg/kube"
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
	err := refreshLocalChecks(ctx)
	if err != nil {
		logrus.Warnf("An error occured refreshing the local cache of checks: %+v", err)
	}

	client := kube.GetKubeClient()
	checkInstances, err := client.DynamicInterface.Resource(instanceGvr).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	logrus.Infof("Found %d checks", len(checkInstances.Items))

	return processAllChecks(ctx, checkInstances.Items)
}

func processAllChecks(ctx context.Context, checkInstances []unstructured.Unstructured) ([]ActionItem, error) {
	actionItems := make([]ActionItem, 0)
	client := kube.GetKubeClient()

	for _, checkInstance := range checkInstances {
		var checkInstanceObject customCheckInstance
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(checkInstance.Object, &checkInstanceObject)
		if err != nil {
			return nil, err
		}
		logrus.Infof("Starting to process check: %s", checkInstanceObject.Name)
		check, err := client.DynamicInterface.Resource(checkGvr).Namespace(checkInstanceObject.Namespace).Get(ctx, checkInstanceObject.Spec.CustomCheckName, metav1.GetOptions{})
		if err != nil {
			actionItems = append(actionItems, ActionItem{
				EventType:         "no-check",
				ResourceName:      checkInstanceObject.Name,
				ResourceNamespace: checkInstanceObject.Namespace,
				ResourceKind:      instanceGvr.Resource,
				Title:             "An error occured retrieving the Custom Check for this instance",
				Remediation:       "Make sure that the custom check exists and it is in the same namespace as this instance.",
				Severity:          0.4,
				Category:          "Reliability",
			})
			logrus.Warnf("Error while retrieving check %+v", err)
			continue
		}
		var checkObject customCheck
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(check.Object, &checkObject)
		if err != nil {
			return nil, err
		}

		newItems, err := processCheck(ctx, checkObject, checkInstanceObject)
		if err != nil {
			return nil, err
		}
		actionItems = append(actionItems, newItems...)
	}
	return actionItems, nil
}

func processCheck(ctx context.Context, check customCheck, checkInstance customCheckInstance) ([]ActionItem, error) {
	actionItems := make([]ActionItem, 0)

	for _, gk := range getGroupKinds(checkInstance.Spec.Targets) {
		newAI, err := processCheckTarget(ctx, check, checkInstance, gk)
		if err != nil {
			return nil, err
		}

		actionItems = append(actionItems, newAI...)
	}

	return actionItems, nil
}

func processCheckTarget(ctx context.Context, check customCheck, checkInstance customCheckInstance, gk schema.GroupKind) ([]ActionItem, error) {
	client := kube.GetKubeClient()
	actionItems := make([]ActionItem, 0)
	mapping, err := client.RestMapper.RESTMapping(gk)
	if err != nil {
		r := fmt.Sprintf("Make sure that the instance targets are correct %s/%s.", gk.Group, gk.Kind)
		actionItems = append(actionItems, ActionItem{
			EventType:         "api-version",
			ResourceName:      checkInstance.Name,
			ResourceNamespace: checkInstance.Namespace,
			ResourceKind:      instanceGvr.Resource,
			Title:             "An error occured retrieving the API Version for this instance",
			Remediation:       r,
			Severity:          0.4,
			Category:          "Reliability",
		})
		logrus.Warnf("Error while retrieving API Version %+v", err)
		return actionItems, nil
	}
	gvr := mapping.Resource
	list, err := client.DynamicInterface.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, obj := range list.Items {
		results, err := runRegoForItem(ctx, check.Spec.Rego, checkInstance.Spec.Parameters, obj.Object)
		if err != nil {
			return nil, err
		}
		aiDetails := outputFormat{}
		aiDetails.SetDefaults(check.Spec.Output, checkInstance.Spec.Output)
		newItems, err := processResults(obj, results, checkInstance.Name, aiDetails)
		actionItems = append(actionItems, newItems...)
	}
	return actionItems, nil
}

func runRegoForItem(ctx context.Context, regoStr string, params map[string]interface{}, obj map[string]interface{}) (rego.ResultSet, error) {
	dataFunction := getKubernetesDataFunction(ctx)
	query, err := rego.New(
		rego.Query("results = data"),
		rego.Module("fairwinds", regoStr),
		rego.Function2(
			&rego.Function{
				Name: "kubernetes",
				Decl: types.NewFunction(types.Args(types.S, types.S), types.S),
			},
			dataFunction)).PrepareForEval(ctx)
	if err != nil {
		return nil, err
	}
	if params == nil {
		params = map[string]interface{}{}
	}

	// TODO Find another way to get parameters in - Should they be a function or input?
	obj["parameters"] = params

	evaluatedInput := rego.EvalInput(obj)
	return query.Eval(ctx, evaluatedInput)
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

func refreshLocalChecks(ctx context.Context) error {
	client := kube.GetKubeClient()
	logrus.Infof("Reconciling checks with Insights backend")
	thisNamespace := "insights-agent"
	namespaceBytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err == nil { //Ignore errors because that means this isn't running in a container
		thisNamespace = string(namespaceBytes)
	}

	checkClient := client.DynamicInterface.Resource(checkGvr)
	instanceClient := client.DynamicInterface.Resource(instanceGvr)

	checkInstances, err := instanceClient.Namespace(thisNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	checks, err := checkClient.Namespace(thisNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	jsonResponse, err := getInsightsChecks()
	if err != nil {
		return err
	}

	logrus.Infof("Found %d checks in Insights, and %d checks in the cluster", len(jsonResponse.Checks), len(checks.Items))
	logrus.Infof("Found %d check instances in Insights, and %d check instances in the cluster", len(jsonResponse.Instances), len(checkInstances.Items))

	logrus.Infof("Deleting stale checks from the cluster")
	for _, check := range checks.Items {
		found := false
		for _, supposedCheck := range jsonResponse.Checks {
			if check.GetName() == supposedCheck.Name {
				found = true
				break
			}
		}
		if !found {
			err = checkClient.Namespace(check.GetNamespace()).Delete(ctx, check.GetName(), metav1.DeleteOptions{})
			if err != nil {
				return err
			}

		}
	}

	logrus.Infof("Deleting stale check instances from the cluster")
	for _, instance := range checkInstances.Items {
		found := false
		for _, supposedInstance := range jsonResponse.Instances {
			if instance.GetName() == supposedInstance.AdditionalData.Name {
				found = true
				break
			}
		}
		if !found {
			err = instanceClient.Namespace(instance.GetNamespace()).Delete(ctx, instance.GetName(), metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
	}

	logrus.Infof("Updating checks in the cluster")
	for _, supposedCheck := range jsonResponse.Checks {
		found := false
		for _, check := range checks.Items {
			if check.GetName() == supposedCheck.Name {
				found = true
				break
			}
		}
		newCheck := supposedCheck.GetUnstructuredObject(thisNamespace)
		if found {
			err = checkClient.Namespace(thisNamespace).Delete(ctx, supposedCheck.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}
		_, err = checkClient.Namespace(thisNamespace).Create(ctx, newCheck, metav1.CreateOptions{})
		if err != nil {
			return err
		}

	}

	logrus.Infof("Updating check instances in the cluster")
	for _, supposedInstance := range jsonResponse.Instances {
		found := false
		for _, instance := range checkInstances.Items {
			if instance.GetName() == supposedInstance.AdditionalData.Name {
				found = true
				break
			}
		}
		newInstance := supposedInstance.GetUnstructuredObject(thisNamespace)
		if found {

			err = instanceClient.Namespace(thisNamespace).Delete(ctx, supposedInstance.AdditionalData.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
		}

		_, err = instanceClient.Namespace(thisNamespace).Create(ctx, newInstance, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
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
	fmt.Println("key", key, m[key], reflect.TypeOf(m[key]))
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

func getDetailsFromMap(m map[string]interface{}) (outputFormat, error) {
	output := outputFormat{}
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

func processResults(resource unstructured.Unstructured, results rego.ResultSet, name string, details outputFormat) ([]ActionItem, error) {
	actionItems := make([]ActionItem, 0)
	for _, output := range getOutputArray(results) {
		strMethod, ok := output.(string)
		outputDetails := outputFormat{}
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
		outputDetails.SetDefaults(details, outputFormat{
			Severity:    &defaultSeverity,
			Title:       &defaultTitle,
			Remediation: &defaultRemediation,
			Category:    &defaultCategory,
			Description: &defaultDescription,
		})

		actionItems = append(actionItems, ActionItem{
			EventType:         name,
			ResourceNamespace: resource.GetNamespace(),
			ResourceKind:      resource.GetKind(),
			ResourceName:      resource.GetName(),
			Title:             *outputDetails.Title,
			Description:       *outputDetails.Description,
			Remediation:       *outputDetails.Remediation,
			Severity:          *outputDetails.Severity,
			Category:          *outputDetails.Category,
		})
	}

	return actionItems, nil
}
