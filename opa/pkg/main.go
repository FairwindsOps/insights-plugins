package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/types"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

const outputFile = "/output/opa.json"

var instanceGvr = schema.GroupVersionResource{Group: "insights.fairwinds.com", Version: "v1beta1", Resource: "customcheckinstances"}
var checkGvr = schema.GroupVersionResource{Group: "insights.fairwinds.com", Version: "v1beta1", Resource: "customchecks"}

func main() {
	logrus.Info("Starting OPA reporter")
	ctx := context.Background()
	client, err := getKubeClient()
	if err != nil {
		panic(err)
	}

	err = refreshLocalChecks(ctx, client.dynamicInterface)
	if err != nil {
		logrus.Warnf("An error occured refreshing the local cache of checks: %+v", err)
	}

	checkInstances, err := client.dynamicInterface.Resource(instanceGvr).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		panic(err)
	}
	logrus.Infof("Found %d checks", len(checkInstances.Items))

	actionItems, err := processAllChecks(ctx, checkInstances.Items, *client)
	if err != nil {
		panic(err)
	}
	logrus.Info("Finished processing OPA checks")

	outputFormat := Output{ActionItems: actionItems}
	value, err := json.Marshal(outputFormat)
	if err != nil {
		panic(err)
	}

	err = ioutil.WriteFile(outputFile, value, 0644)
	if err != nil {
		panic(err)
	}
}

func processAllChecks(ctx context.Context, checkInstances []unstructured.Unstructured, client kubeClient) ([]ActionItem, error) {
	actionItems := make([]ActionItem, 0)

	for _, checkInstance := range checkInstances {
		var checkInstanceObject customCheckInstance
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(checkInstance.Object, &checkInstanceObject)
		if err != nil {
			return nil, err
		}
		logrus.Infof("Starting to process check: %s", checkInstanceObject.Name)
		check, err := client.dynamicInterface.Resource(checkGvr).Namespace(checkInstanceObject.Namespace).Get(ctx, checkInstanceObject.Spec.CustomCheckName, metav1.GetOptions{})
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

		newItems, err := processCheck(ctx, checkObject, checkInstanceObject, client)
		if err != nil {
			return nil, err
		}
		actionItems = append(actionItems, newItems...)
	}
	return actionItems, nil
}

func processCheck(ctx context.Context, check customCheck, checkInstance customCheckInstance, client kubeClient) ([]ActionItem, error) {
	actionItems := make([]ActionItem, 0)

	for _, gk := range getGroupKinds(checkInstance.Spec.Targets) {
		newAI, err := processCheckTarget(ctx, check, checkInstance, gk, client)
		if err != nil {
			return nil, err
		}

		actionItems = append(actionItems, newAI...)
	}

	return actionItems, nil
}

func processCheckTarget(ctx context.Context, check customCheck, checkInstance customCheckInstance, gk schema.GroupKind, client kubeClient) ([]ActionItem, error) {
	actionItems := make([]ActionItem, 0)
	getKubernetesData := getKubernetesDataFunction(ctx, check, client)
	mapping, err := client.restMapper.RESTMapping(gk)
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
	list, err := client.dynamicInterface.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		// TODO look for RBAC errors and create an Action Item
		return nil, err
	}
	for _, obj := range list.Items {
		query, err := rego.New(
			rego.Query("results = data"),
			rego.Module("fairwinds", check.Spec.Rego),
			rego.Function2(
				&rego.Function{
					Name: "kubernetes",
					Decl: types.NewFunction(types.Args(types.S), types.S),
				},
				getKubernetesData),
		).PrepareForEval(ctx)
		if err != nil {
			actionItems = append(actionItems, ActionItem{
				EventType:         "rego-parsing",
				ResourceName:      checkInstance.Name,
				ResourceNamespace: checkInstance.Namespace,
				ResourceKind:      instanceGvr.Resource,
				Title:             "An error occured parsing the Rego for this custom check",
				Remediation:       "Make sure that the Rego is valid for this check.",
				Severity:          0.4,
				Category:          "Reliability",
			})
			logrus.Warnf("Error while parsing Rego %+v", err)
			return actionItems, nil
		}
		if checkInstance.Spec.Parameters != nil {
			obj.Object["parameters"] = checkInstance.Spec.Parameters
		} else {
			obj.Object["parameters"] = map[string]interface{}{}
		}
		// TODO Find another way to get parameters in - Should they be a function or input?
		// TODO Caching
		evaluatedInput := rego.EvalInput(obj.Object)
		results, err := query.Eval(ctx, evaluatedInput)
		if err != nil {
			// Runtime error of Rego
			return nil, err
		}
		newItems, err := processResults(obj, results, check, checkInstance)
		if err != nil {
			actionItems = append(actionItems, ActionItem{
				EventType:         "results-parsing",
				ResourceName:      checkInstance.Name,
				ResourceNamespace: checkInstance.Namespace,
				ResourceKind:      instanceGvr.Resource,
				Title:             "An error occured processing the results of this check.",
				Remediation:       "Make sure that the return values are all correct.",
				Severity:          0.4,
				Category:          "Reliability",
			})
			logrus.Warnf("Error while parsing results %+v", err)
			return actionItems, nil
		}
		actionItems = append(actionItems, newItems...)
	}

	return actionItems, nil
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

func refreshLocalChecks(ctx context.Context, dynamicInterface dynamic.Interface) error {
	logrus.Infof("Reconciling checks with Insights backend")
	thisNamespace := "insights-agent"
	namespaceBytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err == nil { //Ignore errors because that means this isn't running in a container
		thisNamespace = string(namespaceBytes)
	}

	checkClient := dynamicInterface.Resource(checkGvr)
	instanceClient := dynamicInterface.Resource(instanceGvr)

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

func processResults(resource unstructured.Unstructured, results rego.ResultSet, check customCheck, checkInstance customCheckInstance) ([]ActionItem, error) {
	actionItems := make([]ActionItem, 0)
	instanceOutput := checkInstance.Spec.Output
	checkOutput := check.Spec.Output
	for _, output := range getOutputArray(results) {
		severity := checkOutput.Severity
		title := checkOutput.Title
		remediation := checkOutput.Remediation
		category := checkOutput.Category
		if instanceOutput.Severity != nil {
			severity = instanceOutput.Severity
		}
		if instanceOutput.Title != nil {
			title = instanceOutput.Title
		}
		if instanceOutput.Remediation != nil {
			remediation = instanceOutput.Remediation
		}
		if instanceOutput.Category != nil {
			category = instanceOutput.Category
		}
		strMethod, ok := output.(string)
		var description string
		if ok {
			description = strMethod
		} else {
			mapMethod, ok := output.(map[string]interface{})
			if ok {
				description, ok = mapMethod["description"].(string)
				if !ok {
					return nil, errors.New("description was not a string")
				}
				if mapMethod["severity"] != nil {
					severityString, ok := mapMethod["severity"].(string)
					if !ok {
						return nil, errors.New("severity was not a string")
					}
					severityFloat, err := strconv.ParseFloat(severityString, 64)
					if err != nil {
						return nil, err
					}
					severity = &severityFloat
				}
				if mapMethod["title"] != nil {
					titleString, ok := mapMethod["title"].(string)
					if !ok {
						return nil, errors.New("title was not a string")
					}
					title = &titleString
				}

				if mapMethod["remediation"] != nil {
					remediationString, ok := mapMethod["remediation"].(string)
					if !ok {
						return nil, errors.New("remediation was not a string")
					}
					remediation = &remediationString

				}
			} else {
				return nil, fmt.Errorf("could not decipher output format of %+v %T", output, output)
			}
		}
		if severity == nil {
			var severityFloat float64 = 0.0
			severity = &severityFloat
		}
		if title == nil {
			newTitle := "Unknown Title"
			title = &newTitle
		}
		if remediation == nil {
			newRemediation := ""
			remediation = &newRemediation
		}
		if category == nil {
			newCategory := "Reliability"
			category = &newCategory
		}

		actionItems = append(actionItems, ActionItem{
			ResourceNamespace: resource.GetNamespace(),
			ResourceKind:      resource.GetKind(),
			ResourceName:      resource.GetName(),
			Title:             *title,
			EventType:         checkInstance.Name,
			Description:       description,
			Remediation:       *remediation,
			Severity:          *severity,
			Category:          *category,
		})
	}

	return actionItems, nil
}
