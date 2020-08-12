package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/types"
	"github.com/sirupsen/logrus"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	ctrl "sigs.k8s.io/controller-runtime"
)

const outputFile = "/output/opa.json"

var instanceGvr = schema.GroupVersionResource{Group: "insights.fairwinds.com", Version: "v1beta1", Resource: "customcheckinstances"}
var checkGvr = schema.GroupVersionResource{Group: "insights.fairwinds.com", Version: "v1beta1", Resource: "customchecks"}

func main() {
	ctx := context.Background()
	config, err := ctrl.GetConfig()
	if err != nil {
		panic(err)
	}
	dynamicInterface, err := dynamic.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	kube, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	groupResources, err := restmapper.GetAPIGroupResources(kube.Discovery())
	if err != nil {
		panic(err)
	}
	restMapper := restmapper.NewDiscoveryRESTMapper(groupResources)
	// TODO filter by namespace
	checkInstances, err := dynamicInterface.Resource(instanceGvr).Namespace("").List(ctx, metav1.ListOptions{})

	if err != nil {
		panic(err)
	}
	actionItems, err := processAllChecks(ctx, checkInstances.Items, restMapper, dynamicInterface)

	if err != nil {
		panic(err)
	}
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

func processAllChecks(ctx context.Context, checkInstances []unstructured.Unstructured, restMapper meta.RESTMapper, dynamicInterface dynamic.Interface) ([]ActionItem, error) {

	actionItems := make([]ActionItem, 0)

	for _, checkInstance := range checkInstances {
		var checkInstanceObject customCheckInstance
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(checkInstance.Object, &checkInstanceObject)
		if err != nil {
			return nil, err
		}
		logrus.Infof("Starting to process check: %s", checkInstanceObject.Name)
		check, err := dynamicInterface.Resource(checkGvr).Namespace(checkInstanceObject.Namespace).Get(ctx, checkInstanceObject.Spec.CustomCheckName, metav1.GetOptions{})
		if err != nil {
			actionItems = append(actionItems, ActionItem{
				Title:       checkInstanceObject.Name,
				Description: "An error occured retrieving the Custom Check for this instance",
				Remediation: "Make sure that the custom check exists and it is in the same namespace as this instance.",
				Severity:    0.4,
				Category:    "Reliability",
			})
			logrus.Warnf("Error while retrieving check %+v", err)
			continue
		}
		var checkObject customCheck
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(check.Object, &checkObject)
		if err != nil {
			return nil, err
		}

		newItems, err := processCheck(ctx, checkObject, checkInstanceObject, restMapper, dynamicInterface)
		if err != nil {
			return nil, err
		}
		actionItems = append(actionItems, newItems...)
	}
	return actionItems, nil
}

func getKubernetesDataFunction(ctx context.Context, check customCheck, restMapper meta.RESTMapper, dynamicInterface dynamic.Interface) func(rego.BuiltinContext, *ast.Term) (*ast.Term, error) {
	return func(_ rego.BuiltinContext, a *ast.Term) (*ast.Term, error) {

		str, ok := a.Value.(ast.String)
		if !ok {
			return nil, nil
		}
		strValue := str.String()
		if len(strValue) > 0 && strValue[0] == '"' {
			strValue = strValue[1:]
		}
		if len(strValue) > 0 && strValue[len(strValue)-1] == '"' {
			strValue = strValue[:len(strValue)-1]
		}
		var apiGroup string
		for _, target := range check.Spec.AdditionalKubernetesData {
			fmt.Printf("Checking if %s matches %s", strValue, target.Kinds[0])
			if strValue == target.Kinds[0] {
				fmt.Printf("It does!")
				apiGroup = target.APIGroups[0]
				break
			}
		}
		mapping, err := restMapper.RESTMapping(schema.GroupKind{Group: apiGroup, Kind: strValue})
		if err != nil {
			return nil, err
		}
		gvr := mapping.Resource
		list, err := dynamicInterface.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		items := make([]interface{}, 0)
		for _, item := range list.Items {
			items = append(items, item.Object)
		}
		itemValue, err := ast.InterfaceToValue(items)
		if err != nil {
			return nil, err
		}

		return ast.NewTerm(itemValue), nil
	}

}
func processCheck(ctx context.Context, check customCheck, checkInstance customCheckInstance, restMapper meta.RESTMapper, dynamicInterface dynamic.Interface) ([]ActionItem, error) {
	actionItems := make([]ActionItem, 0)

	getKubernetesData := getKubernetesDataFunction(ctx, check, restMapper, dynamicInterface)
	for _, target := range checkInstance.Spec.Targets {
		for _, apiGroup := range target.APIGroups {
			for _, kind := range target.Kinds {
				logrus.Infof("Starting to process %s %s\n\n", apiGroup, kind)
				mapping, err := restMapper.RESTMapping(schema.GroupKind{Group: apiGroup, Kind: kind})
				if err != nil {
					actionItems = append(actionItems, ActionItem{
						Title:       checkInstance.Name,
						Description: "An error occured retrieving the API Version for this instance",
						Remediation: fmt.Sprintf("Make sure that the instance targets are correct %s/%s.", apiGroup, kind),
						Severity:    0.4,
						Category:    "Reliability",
					})
					logrus.Warnf("Error while retrieving API Version %+v", err)
					return actionItems, nil
				}
				gvr := mapping.Resource
				list, err := dynamicInterface.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
				if err != nil {
					return nil, err
				}
				for _, obj := range list.Items {
					query, err := rego.New(
						rego.Query("results = data"),
						rego.Module("fairwinds", check.Spec.Rego),
						rego.Function1(
							&rego.Function{
								Name: "kubernetes",
								Decl: types.NewFunction(types.Args(types.S), types.S),
							},
							getKubernetesData),
					).PrepareForEval(ctx)
					if err != nil {
						actionItems = append(actionItems, ActionItem{
							Title:       checkInstance.Name,
							Description: "An error occured parsing the Rego for this custom check",
							Remediation: "Make sure that the Rego is valid for this check.",
							Severity:    0.4,
							Category:    "Reliability",
						})
						logrus.Warnf("Error while parsing Rego %+v", err)
						return actionItems, nil
					}
					obj.Object["parameters"] = checkInstance.Spec.Parameters
					// TODO Find another way to get parameters in - Should they be a function or input?
					// TODO Caching
					evaluatedInput := rego.EvalInput(obj.Object)
					results, err := query.Eval(ctx, evaluatedInput)
					if err != nil {
						return nil, err
					}
					newItems, err := processResults(obj, results, check, checkInstance)
					if err != nil {
						actionItems = append(actionItems, ActionItem{
							Title:       checkInstance.Name,
							Description: "An error occured processing the results of this check.",
							Remediation: "Make sure that the return values are all correct.",
							Severity:    0.4,
							Category:    "Reliability",
						})
						logrus.Warnf("Error while parsing results %+v", err)
						return actionItems, nil
					}
					actionItems = append(actionItems, newItems...)
				}
			}
		}
	}
	return actionItems, nil
}

func getOutputArray(results rego.ResultSet) []interface{} {
	returnSet := make([]interface{}, 0)

	for _, result := range results {
		for _, pack := range result.Bindings["results"].(map[string]interface{}) {
			for _, outputArray := range pack.(map[string]interface{}) {
				for _, output := range outputArray.([]interface{}) {
					returnSet = append(returnSet, output)
				}
			}
		}
	}
	return returnSet
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
				description = mapMethod["description"].(string)
				if mapMethod["severity"] != nil {
					severityFloat, err := strconv.ParseFloat(mapMethod["severity"].(string), 64)
					if err != nil {
						return nil, err
					}
					severity = &severityFloat
				}
				if mapMethod["title"] != nil {
					titleString := mapMethod["title"].(string)
					title = &titleString
				}

				if mapMethod["remediation"] != nil {
					remediationString := mapMethod["remediation"].(string)
					remediation = &remediationString

				}
			} else {
				return nil, fmt.Errorf("Could not decipher output format of %+v %T", output, output)
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
			Description:       description,
			Remediation:       *remediation,
			Severity:          *severity,
			Category:          *category,
		})
	}

	return actionItems, nil
}
